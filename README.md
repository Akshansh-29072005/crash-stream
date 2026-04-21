# Traffic Accident Detection MVP

This project is a minimal viable product (MVP) for detecting traffic accidents using a combination of a Go backend and a Python backend that utilizes a PyTorch model.

## Project Structure

```
traffic-accident-detection-mvp
├── go-backend
│   ├── cmd
│   │   └── server
│   │       └── main.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── python-backend
│   ├── app
│   │   └── main.py
│   ├── requirements.txt
│   └── Dockerfile
├── docker-compose.yml
├── .env.example
└── README.md
```

## Getting Started

### Prerequisites

- Docker
- Docker Compose

### Setup

1. Clone the repository:
   ```
   git clone <repository-url>
   cd traffic-accident-detection-mvp
   ```

2. Build and run the application using Docker Compose:
   ```
   docker-compose up --build
   ```

### Backends

- **Go Backend**: The Go backend serves as the main server handling requests. It is located in the `go-backend` directory.
- **Python Backend**: The Python backend integrates with a PyTorch model for traffic accident detection. It is located in the `python-backend` directory.

### Usage

Once the application is running, you can send requests to the Go backend, which will process them and interact with the Python backend for accident detection.

### Environment Variables

Refer to the `.env.example` file for the necessary environment variables. Create a `.env` file in the root directory and populate it with your values.

## Contributing

Contributions are welcome! Please open an issue or submit a pull request for any improvements or features.

## License

This project is licensed under the MIT License. See the LICENSE file for details.