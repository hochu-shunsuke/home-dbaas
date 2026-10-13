// Package garage は Garage の管理 API（v2）と S3 API のうち、DB 1つ分のバケットとキーを
// 払い出し・片付けするのに必要な分だけを包む。
package garage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Region は Garage の s3_region と合わせる（infra/garage/garage.yaml）。
const Region = "us-east-1"

// ErrNotFound は対象のバケットやキーが無いことを表す。
var ErrNotFound = errors.New("garage: not found")

// Client は Garage の管理 API と S3 API の入口を持つ。
type Client struct {
	AdminURL string // 例: http://garage.garage.svc:3903
	S3URL    string // 例: http://garage.garage.svc:3900
	Token    string
	HTTP     *http.Client
}

// Key はアクセスキー。SecretAccessKey は GetKey(..., withSecret=true) のときだけ入る。
type Key struct {
	AccessKeyID     string `json:"accessKeyId"`
	Name            string `json:"name"`
	SecretAccessKey string `json:"secretAccessKey,omitempty"`
}

// Bucket はバケット。
type Bucket struct {
	ID            string   `json:"id"`
	GlobalAliases []string `json:"globalAliases"`
}

func (c *Client) call(ctx context.Context, op string, query url.Values, body, out any) error {
	u := c.AdminURL + "/v2/" + op
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	method, reader := http.MethodGet, io.Reader(nil)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		method, reader = http.MethodPost, bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case resp.StatusCode >= 300:
		// 応答本文に秘密は含まれない（秘密を返すのは成功時の GetKeyInfo だけ）
		return fmt.Errorf("garage %s: %s: %s", op, resp.Status, bytes.TrimSpace(raw))
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}

// EnsureBucket は globalAlias が name のバケットを返す。無ければ作る。
func (c *Client) EnsureBucket(ctx context.Context, name string) (*Bucket, error) {
	b, err := c.GetBucket(ctx, name)
	if err == nil {
		return b, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	var created Bucket
	if err := c.call(ctx, "CreateBucket", nil, map[string]any{"globalAlias": name}, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// GetBucket は globalAlias でバケットを引く。
func (c *Client) GetBucket(ctx context.Context, name string) (*Bucket, error) {
	var b Bucket
	if err := c.call(ctx, "GetBucketInfo", url.Values{"globalAlias": {name}}, nil, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// GetKey は名前が name と完全に一致するキーを引く。search は部分一致なので名前を確かめる。
func (c *Client) GetKey(ctx context.Context, name string, withSecret bool) (*Key, error) {
	q := url.Values{"search": {name}}
	if withSecret {
		q.Set("showSecretKey", "true")
	}
	var k Key
	if err := c.call(ctx, "GetKeyInfo", q, nil, &k); err != nil {
		return nil, err
	}
	if k.Name != name {
		return nil, ErrNotFound
	}
	return &k, nil
}

// EnsureKey は name のキーを返す（秘密鍵つき）。無ければ作る。
func (c *Client) EnsureKey(ctx context.Context, name string) (*Key, error) {
	k, err := c.GetKey(ctx, name, true)
	if err == nil {
		return k, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	var created Key
	if err := c.call(ctx, "CreateKey", nil, map[string]any{"name": name}, &created); err != nil {
		return nil, err
	}
	return &created, nil
}

// Allow はキーにバケットの読み書きを許す。何度呼んでも同じ。
func (c *Client) Allow(ctx context.Context, bucketID, accessKeyID string) error {
	return c.call(ctx, "AllowBucketKey", nil, map[string]any{
		"bucketId":    bucketID,
		"accessKeyId": accessKeyID,
		"permissions": map[string]bool{"read": true, "write": true, "owner": true},
	}, nil)
}

// DeleteBucketAndKey はバケットを空にしてから消し、キーも消す。既に無ければ何もしない。
// 中身を消すのには、そのバケット専用のキー自身を使う（管理 API には中身を消す操作が無い）。
func (c *Client) DeleteBucketAndKey(ctx context.Context, name string) error {
	key, keyErr := c.GetKey(ctx, name, true)
	if keyErr != nil && !errors.Is(keyErr, ErrNotFound) {
		return keyErr
	}
	bucket, err := c.GetBucket(ctx, name)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return err
	default:
		if key == nil {
			return fmt.Errorf("バケット %s を空にするためのキーが見つからない", name)
		}
		if err := c.emptyBucket(ctx, name, key); err != nil {
			return fmt.Errorf("バケット %s を空にできない: %w", name, err)
		}
		if err := c.call(ctx, "DeleteBucket", url.Values{"id": {bucket.ID}}, map[string]any{}, nil); err != nil &&
			!errors.Is(err, ErrNotFound) {
			return err
		}
	}
	if key != nil {
		return c.call(ctx, "DeleteKey", url.Values{"id": {key.AccessKeyID}}, map[string]any{}, nil)
	}
	return nil
}

func (c *Client) emptyBucket(ctx context.Context, bucket string, key *Key) error {
	s3c := s3.New(s3.Options{
		BaseEndpoint: aws.String(c.S3URL),
		Region:       Region,
		UsePathStyle: true,
		Credentials:  credentials.NewStaticCredentialsProvider(key.AccessKeyID, key.SecretAccessKey, ""),
		// 新しい SDK は既定でチェックサムを付けて送る。S3 互換ストレージで弾かれないよう、必要なときだけにする
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	})
	pages := s3.NewListObjectsV2Paginator(s3c, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return err
		}
		if len(page.Contents) == 0 {
			continue
		}
		ids := make([]types.ObjectIdentifier, 0, len(page.Contents))
		for _, o := range page.Contents {
			ids = append(ids, types.ObjectIdentifier{Key: o.Key})
		}
		if _, err := s3c.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		}); err != nil {
			return err
		}
	}
	return nil
}
