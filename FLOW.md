```mermaid
sequenceDiagram
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