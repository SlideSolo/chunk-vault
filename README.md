# Chunk Vault

Chunk Vault is a simple implementation of a file storage system that allows you to upload, store, and retrieve files. It is designed as a competitor to Amazon S3, enabling you to store files in multiple parts across different storage servers.

## Features

- Upload a file: Upload a file via REST API, which will be split into approximately equal parts and distributed across different storage servers.
- Download a file: Retrieve the complete file by assembling its parts from the storage servers using a REST API.
- Add a new storage server: Dynamically add a new storage server to the system to scale up storage capacity.

## Exposed metrics
- api_requests_count - number of requests to api

## Prerequisites

- Golang (1.16 or later)
- curl or postman (for testing the API endpoints)

## Installation

1. Clone the repository:

   ```shell
   git clone https://github.com/SlideSolo/chunk-vault.git
   cd chunk-vault
    ```

2. Build service
   ```shell
   go build
   ```
   
3. Start the service:
```shell
./chunk-vault
```

The service will listen on http://localhost:8080 by default.
Use a REST client (e.g., cURL, Postman) to interact with the service:

### Upload a file: 
Send a POST request to http://localhost:8080/upload with the file attached as multipart/form-data with the key file.

```shell
curl -X POST -F "file=@/path/to/file.txt" http://localhost:8080/upload
```

Replace "/path/to/file.txt" with the actual path to the file you want to upload.
Once you send the request, the service will receive the file and split it into multiple chunks, which will be stored on the configured storage servers.

### Download a file: 
Send a GET request to http://localhost:8080/download?file=test.txt 
The service will retrieve the file chunks from the storage servers, concatenate them, and provide the file as a download.
Upon successful execution of the request, the file will be downloaded to your local machine.

### Add a storage server: 
Send a GET request to http://localhost:8080/addServer. This will add a new storage server dynamically to the system.

```shell
curl -X POST -d '{"address": "http://localhost:9007"}' http://localhost:8080/addServer
```

This request sends a POST request to the "/addServer" endpoint, triggering the addition of a new storage server dynamically to the system.

## Notes

This service is intended for educational and demonstration purposes. In a real-world scenario, you would need to implement proper data persistence, security measures, and error handling.
For this demonstration, the storage servers are simulated and run on different ports. In practice, you would set up actual storage servers with reliable data storage mechanisms.

## License
This project is licensed under the MIT License - see the LICENSE file for details.

