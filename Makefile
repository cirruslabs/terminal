API_FOLDER=internal/api

all:
	mkdir -p $(API_FOLDER)
	protoc --go_out=$(API_FOLDER) --go_opt=paths=source_relative --go-grpc_out=$(API_FOLDER) --go-grpc_opt=paths=source_relative \
		-I proto/ proto/terminal.proto

clean:
	rm -rf $(API_FOLDER)
