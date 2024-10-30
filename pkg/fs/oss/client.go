package oss

import (
	"github.com/aliyun/aliyun-oss-go-sdk/oss"
)

type Client struct {
	clt    *oss.Client
	bucket *oss.Bucket
}

func NewClient(
	bucketName string,
	endpoint string,
	accessID string,
	accessSecret string,
) (*Client, error) {
	client, err := oss.New(endpoint, accessID, accessSecret)
	if err != nil {
		return nil, err
	}
	bucket, err := client.Bucket(bucketName)
	if err != nil {
		return nil, err
	}

	return &Client{
		clt:    client,
		bucket: bucket,
	}, nil
}

func (clt *Client) list(name string, cb func([]oss.ObjectProperties) error) ([]oss.ObjectProperties, error) {
	var list []oss.ObjectProperties
	prefix := oss.Prefix(clearDirPath(name))
	continuationToken := oss.ContinuationToken("")
	startAfter := oss.StartAfter("")
	for {
		res, err := clt.bucket.ListObjectsV2(prefix, startAfter, continuationToken, oss.MaxKeys(MaxKeys))
		if err != nil {
			return list, err
		}
		list = append(list, res.Objects...)
		if cb != nil {
			if err := cb(list); err != nil {
				return list, err
			}
		}
		if !res.IsTruncated {
			break
		}
		startAfter = oss.StartAfter(res.StartAfter)
		continuationToken = oss.ContinuationToken(res.NextContinuationToken)
	}
	return list, nil
}
