# Userflow
1. user open the website
2. user has account?
    - if user has account -> user login
    - if user doesn't have account -> user register
3. user login
    - if user credentials invalid -> send {type: "auth_failed"} to client
    - if user credentials valid
        - send {type: "auth_success", username: "..."} to client
        - broadcast {type: "user_join", username: "..."} to all clients
4. retrieve history -> http GET /messages?token=...
5. user send message -> http POST /messages?token=... {message: "..."}
6. user leave/close the website
    - broadcast {type: "user_leave", username: "..."} to all clients
    - close the websocket connection

## Diagram
### WebSocket Connection
```mermaid
    sequenceDiagram
        title WebSocket Connection
        participant Client
        participant Service
        participant DB
        
        %% Authentication Flow
        Client->>Service: Connect to /ws endpoint
        Service->>Service: Upgrade HTTP to WebSocket
        Client->>Service: Send auth message {type: "auth", token: "..."}
        Service->>DB: ValidateSession(token)
        
        alt Invalid Auth
            Service->>Client: {type: "auth_failed"}
            Service->>Client: Close connection
        else Valid Auth
            Service->>Client: {type: "auth_success", username: "..."}
            Service->>Client: Broadcast {type: "user_join", username: "..."}
            
            %% Message Flow
            rect rgb(120, 140, 175)
                note right of Client: Chat Loop
                Client->>Service: Send message {type: "message", content: "..."}
                Service->>DB: StoreMessage(userID, content)
                Service->>Client: Broadcast to all clients
            end
            
            %% Disconnect Flow
            Client->>Service: Connection closed
            Service->>Client: Broadcast {type: "user_leave", username: "..."}
            Service->>Service: Remove client from sync.Map
        end
```