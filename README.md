# TRH Backend

This is the backend service for the TRH application. It is built using Go and utilizes the Gin framework for handling HTTP requests and GORM for database interactions.

## Getting Started

### Prerequisites

- Go 1.22.6 which is compatible with the version of TRH SDK
- PostgreSQL

### Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/tokamak-network/trh-backend.git
   cd trh-backend
   ```

2. Copy the example environment file and configure it:
   ```bash
   cp .env.example .env
   ```

3. Update the `.env` file with your database credentials and other configurations.

### Running the Application

1. Ensure your PostgreSQL server is running and accessible.
   You can use the docker compose file to start a local postgres server.
   ```bash
   docker compose up -d
   ```

2. Run the application:
   ```bash
   go run main.go
   ```

3. The server will start on the port specified in the `.env` file (default is 8000).

### Contributing

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/YourFeature`).
3. Commit your changes (`git commit -am 'Add new feature'`).
4. Push to the branch (`git push origin feature/YourFeature`).
5. Create a new Pull Request.

### API Documentation
The swagger API documentation is running at the endpoint
```
http://localhost:${PORT}/swagger/index.html
```


### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

### Acknowledgments

- Thanks to the contributors of the open-source libraries used in this project.
