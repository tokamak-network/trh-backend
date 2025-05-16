# TRH Backend

This is the backend service for the TRH application. It is built using Go and utilizes the Gin framework for handling HTTP requests and GORM for database interactions.

## Getting Started

### Prerequisites

- Go 1.23 or later
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

2. Run the application:
   ```bash
   go run ./cmd
   ```

3. The server will start on the port specified in the `.env` file (default is 8000).

### Project Structure

- `server/`: Contains the main server code.
- `go.mod` and `go.sum`: Go module files for dependency management.
- `.env` and `.env.example`: Environment configuration files.

### Contributing

1. Fork the repository.
2. Create a new branch (`git checkout -b feature/YourFeature`).
3. Commit your changes (`git commit -am 'Add new feature'`).
4. Push to the branch (`git push origin feature/YourFeature`).
5. Create a new Pull Request.

### License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

### Acknowledgments

- Thanks to the contributors of the open-source libraries used in this project.
