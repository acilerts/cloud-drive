package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioClient interface {
	BucketExists(
		ctx context.Context,
		bucketName string,
	) (bool, error)

	MakeBucket(
		ctx context.Context,
		bucketName string,
		opts minio.MakeBucketOptions,
	) error

	PutObject(
		ctx context.Context,
		bucketName string,
		objectName string,
		reader io.Reader,
		objectSize int64,
		opts minio.PutObjectOptions,
	) (minio.UploadInfo, error)

	RemoveObject(
		ctx context.Context,
		bucketName string,
		objectName string,
		opts minio.RemoveObjectOptions,
	) error
}

type ObjectStore struct {
	client minioClient
	bucket string
}

func NewObjectStore(
	ctx context.Context,
	endpoint string,
	accessKey string,
	secretKey string,
	bucket string,
) (*ObjectStore, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds: credentials.NewStaticV4(
			accessKey,
			secretKey,
			"",
		),
		Secure: false,
	})
	if err != nil {
		return nil, fmt.Errorf("create minio client: %w", err)
	}

	return newObjectStore(ctx, client, bucket)
}

func newObjectStore(
	ctx context.Context,
	client minioClient,
	bucket string,
) (*ObjectStore, error) {
	var lastErr error

	for attempt := 1; attempt <= 30; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("wait for minio: %w", err)
		}

		exists, err := client.BucketExists(ctx, bucket)
		if err == nil && exists {
			return &ObjectStore{
				client: client,
				bucket: bucket,
			}, nil
		}

		if err != nil {
			lastErr = err
		} else {
			err = client.MakeBucket(
				ctx,
				bucket,
				minio.MakeBucketOptions{},
			)

			if err == nil {
				return &ObjectStore{
					client: client,
					bucket: bucket,
				}, nil
			}

			lastErr = err
		}

		if attempt == 30 {
			break
		}

		timer := time.NewTimer(time.Second)

		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("wait for minio: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf(
		"minio did not become ready after 30 attempts: %w",
		lastErr,
	)
}

func (store *ObjectStore) Upload(
	ctx context.Context,
	objectKey string,
	reader io.Reader,
	size int64,
	contentType string,
) error {
	_, err := store.client.PutObject(
		ctx,
		store.bucket,
		objectKey,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)

	if err != nil {
		return fmt.Errorf("upload object to minio: %w", err)
	}

	return nil
}

func (store *ObjectStore) Delete(
	ctx context.Context,
	objectKey string,
) error {
	err := store.client.RemoveObject(
		ctx,
		store.bucket,
		objectKey,
		minio.RemoveObjectOptions{},
	)

	if err != nil {
		return fmt.Errorf("delete object from minio: %w", err)
	}

	return nil
}
