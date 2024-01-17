dapr stop --app-id campaign-manager
(lsof -i:8080 | grep main) | awk '{print $2}' | xargs kill