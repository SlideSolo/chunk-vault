package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"reflect"
	"testing"

	"github.com/SlideSolo/chunk-vault/config"

	"github.com/sirupsen/logrus"
)

func TestUploadHandler_Success(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.New(ctx)
	if err != nil {
		logrus.WithError(err).Fatalln("failed to load config")
	}

	l := logrus.New()
	l.SetOutput(io.Discard)
	logger := logrus.NewEntry(l)

	m := new(MetricsMock)

	s := Service{
		cfg:                      cfg,
		logger:                   logger,
		metrics:                  m,
		currentNumStorageServers: cfg.App.NumStorageServers,
	}

	// Create a test server to handle the requests
	testServer := httptest.NewServer(http.HandlerFunc(s.UploadHandler))
	defer testServer.Close()

	// Simulate file upload
	fileContents := "This is a test file."
	fileName := "test.txt"
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", fileName)
	_, err = io.Copy(part, bytes.NewBufferString(fileContents))
	if err != nil {
		t.Fatalf("Failed to copy file contents: %v", err)
	}
	err = writer.Close()
	if err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	request, err := http.NewRequest(http.MethodPost, testServer.URL+"/upload", body)
	if err != nil {
		t.Fatalf("Failed to create a request: %v", err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("Failed to make the request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, but got %d", http.StatusCreated, response.StatusCode)
	}
}

func TestUploadHandler_InvalidRequest(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.New(ctx)
	if err != nil {
		logrus.WithError(err).Fatalln("failed to load config")
	}

	l := logrus.New()
	l.SetOutput(io.Discard)
	logger := logrus.NewEntry(l)

	m := new(MetricsMock)

	s := Service{
		cfg:                      cfg,
		logger:                   logger,
		metrics:                  m,
		currentNumStorageServers: cfg.App.NumStorageServers,
	}

	// Create a test server to handle the requests
	testServer := httptest.NewServer(http.HandlerFunc(s.UploadHandler))
	defer testServer.Close()

	// Simulate an invalid request (missing 'file' form field)
	invalidRequest, err := http.NewRequest(http.MethodPost, testServer.URL+"/upload", nil)
	if err != nil {
		t.Fatalf("Failed to create an invalid request: %v", err)
	}

	// Set the correct Content-Type header for a valid multipart form request
	invalidRequest.Header.Set("Content-Type", "multipart/form-data; boundary=---BOUNDARY")

	client := &http.Client{}
	response, err := client.Do(invalidRequest)
	if err != nil {
		t.Fatalf("Failed to make the invalid request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d for invalid request, but got %d", http.StatusBadRequest, response.StatusCode)
	}
}

func TestDownloadHandler(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	cfg, err := config.New(ctx)
	if err != nil {
		logrus.WithError(err).Fatalln("failed to load config")
	}

	l := logrus.New()
	l.SetOutput(io.Discard)
	logger := logrus.NewEntry(l)

	m := new(MetricsMock)

	s := Service{
		cfg:                      cfg,
		logger:                   logger,
		metrics:                  m,
		currentNumStorageServers: cfg.App.NumStorageServers,
	}

	// Test cases
	tests := []struct {
		name               string
		fileName           string
		filePartsMap       map[string][]FilePart
		expectedStatusCode int
		expectedContent    []byte
	}{
		{
			name:     "File Found",
			fileName: "example.txt",
			filePartsMap: map[string][]FilePart{
				"example.txt": {
					{Data: []byte{1, 2, 3}},
					{Data: []byte{4, 5}},
				},
			},
			expectedStatusCode: http.StatusOK,
			expectedContent:    []byte{1, 2, 3, 4, 5},
		},
		{
			name:               "File Not Found",
			fileName:           "nonexistent.txt",
			filePartsMap:       map[string][]FilePart{},
			expectedStatusCode: http.StatusNotFound,
			expectedContent:    []byte("File not found\n"),
		},
		{
			name:               "Missing File Parameter",
			fileName:           "",
			filePartsMap:       map[string][]FilePart{},
			expectedStatusCode: http.StatusBadRequest,
			expectedContent:    []byte("Please provide the 'file' parameter\n"),
		},
	}

	// Run the test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a mock HTTP request with the given file name
			req, err := http.NewRequest("GET", fmt.Sprintf("/download?file=%s", test.fileName), nil)
			if err != nil {
				t.Fatalf("Failed to create a mock HTTP request: %v", err)
			}

			// Create a mock HTTP response recorder
			rr := httptest.NewRecorder()

			// Set the file parts map in the FilePartsMap global variable for the test case
			FilePartsMap.Lock()
			FilePartsMap.m = test.filePartsMap
			FilePartsMap.Unlock()

			// Call the DownloadHandler function
			s.DownloadHandler(rr, req)

			// Check if the response status code matches the expected status code
			if rr.Code != test.expectedStatusCode {
				t.Errorf("Test case '%s' failed. Expected status code: %d, Got: %d", test.name, test.expectedStatusCode, rr.Code)
			}

			// Check if the response content matches the expected content
			if !reflect.DeepEqual(rr.Body.Bytes(), test.expectedContent) {
				t.Errorf("Test case '%s' failed. Expected content: %v, Got: %v", test.name, test.expectedContent, rr.Body.String())
			}
		})
	}
}

func TestAssembleFile(t *testing.T) {
	// Test cases
	tests := []struct {
		name      string
		fileParts []FilePart
		expected  []byte
	}{
		{
			name: "Single FilePart",
			fileParts: []FilePart{
				{Data: []byte{1, 2, 3}},
			},
			expected: []byte{1, 2, 3},
		},
		{
			name: "Multiple FileParts",
			fileParts: []FilePart{
				{Data: []byte{1, 2}},
				{Data: []byte{3, 4}},
				{Data: []byte{5, 6}},
			},
			expected: []byte{1, 2, 3, 4, 5, 6},
		},
		{
			name:      "Empty FileParts",
			fileParts: []FilePart{},
			expected:  nil,
		},
	}

	// Run the test cases
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &Service{}
			result := service.assembleFile(test.fileParts)

			// Check if the result matches the expected output
			if !reflect.DeepEqual(result, test.expected) {
				t.Errorf("Test case '%s' failed. Expected: %v, Got: %v", test.name, test.expected, result)
			}
		})
	}
}
