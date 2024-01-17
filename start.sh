dapr run --app-id campaign-manager \
         --app-protocol http \
         --app-port 8080 \
         --dapr-http-port 3500 \
         --log-level debug \
         --resources-path ./config \
         go run main.go