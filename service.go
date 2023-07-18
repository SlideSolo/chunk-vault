package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/SlideSolo/chunk-vault/config"

	"github.com/sirupsen/logrus"
)

type metrics interface {
	IncApiRequestsCount(method, endpoint, status string)
}

type Service struct {
	cfg                      *config.Config
	logger                   *logrus.Entry
	metrics                  metrics
	storageServers           []string
	currentNumStorageServers int
}

func NewService(cfg *config.Config, logger *logrus.Entry, m *config.Metrics) *Service {
	return &Service{
		cfg:     cfg,
		logger:  logger,
		metrics: m,
	}
}

type AddServerRequest struct {
	Address string `json:"address"`
}

// FilePart represents a part of the file with its index and data
type FilePart struct {
	Index int    `json:"index"`
	Data  []byte `json:"data"`
}

// FilePartsMap keeps track of file parts for each file
var FilePartsMap = struct {
	sync.RWMutex
	m map[string][]FilePart
}{m: make(map[string][]FilePart)}

func (s *Service) Run() {
	http.HandleFunc("/upload", s.UploadHandler)
	http.HandleFunc("/download", s.DownloadHandler)
	http.HandleFunc("/addServer", s.AddServerHandler)

	// Start the REST server
	go func() {
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	s.logger.Println("REST server started on port 8080")
	s.currentNumStorageServers = s.cfg.App.NumStorageServers

	// Additional steps can be taken to start the storage servers, but they are simulated here for demonstration purposes
	for i := 0; i < s.currentNumStorageServers; i++ {
		http.HandleFunc(fmt.Sprintf("/storage%d", i+1), func(w http.ResponseWriter, r *http.Request) {
			fileName := filepath.Base(r.URL.Path)
			var part FilePart
			err := json.NewDecoder(r.Body).Decode(&part)
			if err != nil {
				http.Error(w, "Failed to decode the file part", http.StatusBadRequest)
				return
			}

			FilePartsMap.Lock()
			FilePartsMap.m[fileName][part.Index] = part
			FilePartsMap.Unlock()
		})

		storageServerAddr := fmt.Sprintf(":900%d", i+1)
		s.logger.Printf("Storage server %d started on address: %s\n", i+1, storageServerAddr)
		go func(addr string) {
			log.Fatal(http.ListenAndServe(addr, nil))
		}(storageServerAddr)
	}

	select {}
}

// UploadHandler handles the file upload and storage
func (s *Service) UploadHandler(w http.ResponseWriter, r *http.Request) {
	file, header, err := r.FormFile("file")
	if err != nil {
		s.metrics.IncApiRequestsCount("POST", "/upload", "400")
		http.Error(w, "Failed to read the uploaded file", http.StatusBadRequest)
		return
	}
	defer func(file multipart.File) {
		err := file.Close()
		if err != nil {
			s.logger.WithError(err).Error("Failed to close the file")
		}
	}(file)

	fileName := header.Filename
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		s.metrics.IncApiRequestsCount("POST", "/upload", "500")
		http.Error(w, "Failed to read the file", http.StatusInternalServerError)
		return
	}

	fileParts := s.splitFile(fileBytes)
	FilePartsMap.Lock()
	FilePartsMap.m[fileName] = fileParts
	FilePartsMap.Unlock()

	// Send the file parts to storage servers
	s.distributeFileParts(fileName, fileParts)
	s.metrics.IncApiRequestsCount("POST", "/upload", "200")
	w.WriteHeader(http.StatusCreated)
}

// DownloadHandler handles the file retrieval
func (s *Service) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		s.metrics.IncApiRequestsCount("GET", "/download", "400")
		http.Error(w, "Please provide the 'file' parameter", http.StatusBadRequest)
		return
	}

	FilePartsMap.RLock()
	fileParts, ok := FilePartsMap.m[fileName]
	FilePartsMap.RUnlock()

	if !ok {
		s.metrics.IncApiRequestsCount("GET", "/download", "404")
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Retrieve file parts from storage servers
	completeFile := s.assembleFile(fileParts)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	_, err := w.Write(completeFile)
	if err != nil {
		s.metrics.IncApiRequestsCount("GET", "/download", "500")
		http.Error(w, "Failed to send the file", http.StatusInternalServerError)
		return
	}
	s.metrics.IncApiRequestsCount("GET", "/download", "200")
}

// AddServerHandler handles the addition of a new storage server
func (s *Service) AddServerHandler(w http.ResponseWriter, r *http.Request) {
	var newServerAddr AddServerRequest

	if r.Method != http.MethodPost {
		s.metrics.IncApiRequestsCount("POST", "/addServer", "405")
		http.Error(w, "Invalid request method. POST method is required.", http.StatusMethodNotAllowed)
		return
	}

	// Read the address of the new storage server from the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.metrics.IncApiRequestsCount("POST", "/addServer", "500")
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			s.logger.WithError(err).Error("Failed to close the request body")
		}
	}(r.Body)

	err = json.Unmarshal(body, &newServerAddr)
	if err != nil {
		s.metrics.IncApiRequestsCount("POST", "/addServer", "400")
		http.Error(w, "Failed to unmarshal request body", http.StatusBadRequest)
		return
	}

	// Add the new storage server address to the list
	s.storageServers = append(s.storageServers, newServerAddr.Address)
	s.currentNumStorageServers += 1
	s.logger.Printf("New storage server added: %s\n", newServerAddr.Address)
	w.WriteHeader(http.StatusCreated)
	s.metrics.IncApiRequestsCount("POST", "/addServer", "200")
}

// splitFile divides the file into approximately equal parts
func (s *Service) splitFile(fileBytes []byte) []FilePart {
	partSize := (len(fileBytes) + s.currentNumStorageServers - 1) / s.currentNumStorageServers
	var fileParts []FilePart

	for i := 0; i < s.currentNumStorageServers; i++ {
		start := i * partSize
		end := (i + 1) * partSize
		if end > len(fileBytes) {
			end = len(fileBytes)
		}

		fileParts = append(fileParts, FilePart{
			Index: i,
			Data:  fileBytes[start:end],
		})
	}

	return fileParts
}

// distributeFileParts sends the file parts to storage servers
func (s *Service) distributeFileParts(fileName string, fileParts []FilePart) {
	for i, part := range fileParts {
		// For simplicity, we'll just print the data of each part here.
		// In a real implementation, you would send the data to the storage servers over the network.
		// Here, we'll assume that the storage servers are listening on ports 9001 to 9006.
		storageServerAddr := fmt.Sprintf("http://localhost:900%d", i+1)
		s.storageServers = append(s.storageServers, storageServerAddr)
		s.logger.Printf("Sending part %d of file %s to storage server: %s\n", part.Index, fileName, storageServerAddr)
	}
}

// assembleFile assembles the file from its parts
func (s *Service) assembleFile(fileParts []FilePart) []byte {
	var completeFile []byte

	for _, part := range fileParts {
		completeFile = append(completeFile, part.Data...)
	}

	return completeFile
}
