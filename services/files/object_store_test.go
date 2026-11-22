package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

type fakeMinioClient struct {
	bucketExists    bool
	bucketExistsErr error
	makeBucketErr   error
	uploadErr       error
	deleteErr       error

	bucketExistsCalls int
	makeBucketCalls   int
	uploadCalls       int
	deleteCalls       int

	bucketName  string
	objectName  string
	content     string
	contentType string
}

func (f *fakeMinioClient) BucketExists(
	ctx context.Context,
	bucketName string,
) (bool, error) {
	f.bucketExistsCalls++
	f.bucketName = bucketName
	return f.bucketExists, f.bucketExistsErr
}

func (f *fakeMinioClient) MakeBucket(
	ctx context.Context,
	bucketName string,
	opts minio.MakeBucketOptions,
) error {
	f.makeBucketCalls++
	f.bucketName = bucketName

	if f.makeBucketErr == nil {
		f.bucketExists = true
	}

	return f.makeBucketErr
}

func (f *fakeMinioClient) PutObject(
	ctx context.Context,
	bucketName string,
	objectName string,
	reader io.Reader,
	objectSize int64,
	opts minio.PutObjectOptions,
) (minio.UploadInfo, error) {
	f.uploadCalls++
	f.bucketName = bucketName
	f.objectName = objectName
	f.contentType = opts.ContentType

	data, err := io.ReadAll(reader)
	if err != nil {
		return minio.UploadInfo{}, err
	}

	f.content = string(data)

	if f.uploadErr != nil {
		return minio.UploadInfo{}, f.uploadErr
	}

	return minio.UploadInfo{}, nil
}

func (f *fakeMinioClient) RemoveObject(
	ctx context.Context,
	bucketName string,
	objectName string,
	opts minio.RemoveObjectOptions,
) error {
	f.deleteCalls++
	f.bucketName = bucketName
	f.objectName = objectName
	return f.deleteErr
}

func TestNewObjectStoreUsesExistingBucket(t *testing.T) {
	client := &fakeMinioClient{
		bucketExists: true,
	}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store == nil {
		t.Fatal("expected object store")
	}

	if client.bucketExistsCalls != 1 {
		t.Fatalf("expected 1 bucket check, got %d", client.bucketExistsCalls)
	}

	if client.makeBucketCalls != 0 {
		t.Fatalf("expected no bucket creation, got %d", client.makeBucketCalls)
	}
}

func TestNewObjectStoreCreatesBucket(t *testing.T) {
	client := &fakeMinioClient{}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if store == nil {
		t.Fatal("expected object store")
	}

	if client.makeBucketCalls != 1 {
		t.Fatalf("expected 1 bucket creation, got %d", client.makeBucketCalls)
	}
}

func TestNewObjectStoreFailsWhenBucketCreationFails(t *testing.T) {
	expectedErr := errors.New("minio unavailable")

	client := &fakeMinioClient{
		makeBucketErr: expectedErr,
	}

	ctx, cancel := context.WithCancel(context.Background())

	cancel()

	_, err := newObjectStore(ctx, client, "cloud-drive")

	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestNewObjectStoreStopsWhenContextIsCanceled(t *testing.T) {
	client := &fakeMinioClient{
		bucketExistsErr: errors.New("minio unavailable"),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := newObjectStore(ctx, client, "cloud-drive")

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}

	if client.bucketExistsCalls != 0 {
		t.Fatalf("expected no bucket checks, got %d", client.bucketExistsCalls)
	}
}

func TestObjectStoreUploadSuccessfully(t *testing.T) {
	client := &fakeMinioClient{
		bucketExists: true,
	}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)
	if err != nil {
		t.Fatalf("create object store: %v", err)
	}

	content := "test file content"

	err = store.Upload(
		context.Background(),
		"user-123/file-456",
		bytes.NewBufferString(content),
		int64(len(content)),
		"text/plain",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.uploadCalls != 1 {
		t.Fatalf("expected 1 upload, got %d", client.uploadCalls)
	}

	if client.bucketName != "cloud-drive" {
		t.Fatalf("unexpected bucket: %s", client.bucketName)
	}

	if client.objectName != "user-123/file-456" {
		t.Fatalf("unexpected object key: %s", client.objectName)
	}

	if client.content != content {
		t.Fatalf("unexpected content: %s", client.content)
	}

	if client.contentType != "text/plain" {
		t.Fatalf("unexpected content type: %s", client.contentType)
	}
}

func TestObjectStoreUploadReturnsError(t *testing.T) {
	expectedErr := errors.New("upload failed")

	client := &fakeMinioClient{
		bucketExists: true,
		uploadErr:    expectedErr,
	}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)
	if err != nil {
		t.Fatalf("create object store: %v", err)
	}

	err = store.Upload(
		context.Background(),
		"file-123",
		strings.NewReader("content"),
		7,
		"text/plain",
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected upload error, got %v", err)
	}
}

func TestObjectStoreDeleteSuccessfully(t *testing.T) {
	client := &fakeMinioClient{
		bucketExists: true,
	}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)
	if err != nil {
		t.Fatalf("create object store: %v", err)
	}

	err = store.Delete(
		context.Background(),
		"user-123/file-456",
	)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if client.deleteCalls != 1 {
		t.Fatalf("expected 1 delete, got %d", client.deleteCalls)
	}

	if client.objectName != "user-123/file-456" {
		t.Fatalf("unexpected object key: %s", client.objectName)
	}
}

func TestObjectStoreDeleteReturnsError(t *testing.T) {
	expectedErr := errors.New("delete failed")

	client := &fakeMinioClient{
		bucketExists: true,
		deleteErr:    expectedErr,
	}

	store, err := newObjectStore(
		context.Background(),
		client,
		"cloud-drive",
	)
	if err != nil {
		t.Fatalf("create object store: %v", err)
	}

	err = store.Delete(
		context.Background(),
		"file-123",
	)

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected delete error, got %v", err)
	}
}
