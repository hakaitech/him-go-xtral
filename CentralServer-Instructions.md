# **Project Himitsu: Component 2 \- Central Server**

This document details the complete technical specification for the Central Server. This server is a "dumb" broker, designed to be simple, fast, and stateless (relying on Redis).

We will use **Go** for the API, **Redis** for the identity database, and **Mosquitto** for the MQTT broker. **Docker Compose** will orchestrate all three services.

## **1\. Directory Structure**

himitsu-server/  
├── docker-compose.yml  
├── api/  
│   ├── Dockerfile  
│   ├── go.mod  
│   └── main.go  
├── mosquitto/  
│   ├── Dockerfile  
│   ├── mosquitto.conf  
│   └── acl.conf  
└── redis/  
    (No files needed, will use official image)

## **2\. Component 1: Go Identity API (api/main.go)**

This service handles hash \<-\> pub\_key mapping.

* **Tech:** Go 1.21+, Gin (or net/http), go-redis.  
* **Dependencies (go.mod):**  
  * github.com/gin-gonic/gin  
  * github.com/redis/go-redis/v9  
  * github.com/golang-jwt/jwt/v5 (or a crypto lib for key verification)  
  * golang.org/x/crypto/bcrypt (or similar, if we add passwords)  
  * **Note:** We need a good Go library for secp256r1 signature verification. crypto/ecdsa is standard.

### **API Endpoints**

package main

import (  
    "context"  
    "log"  
    "net/http"

    "\[github.com/gin-gonic/gin\](https://github.com/gin-gonic/gin)"  
    "\[github.com/redis/go-redis/v9\](https://github.com/redis/go-redis/v9)"  
    // We'll need crypto/ecdsa, crypto/x509, encoding/pem  
)

var ctx \= context.Background()  
var rdb \*redis.Client

// \--- Pydantic-like Models \---  
type RegisterRequest struct {  
    Hash   string \`json:"hash" binding:"required,len=10"\`  
    PubKey string \`json:"pub\_key" binding:"required"\`  
}

type DisposeRequest struct {  
    Hash      string \`json:"hash" binding:"required,len=10"\`  
    Signature string \`json:"signature" binding:"required"\`  
}

// \--- Main Setup \---  
func main() {  
    rdb \= redis.NewClient(\&redis.Options{  
        Addr: "redis:6379", // 'redis' is the Docker service name  
    })

    router := gin.Default()  
      
    router.POST("/register", handleRegister)  
    router.GET("/get\_pub\_key", handleGetKey)  
    router.POST("/dispose", handleDispose)

    log.Fatal(router.Run(":8080"))  
}

// \--- Endpoint Handlers \---

// POST /register  
func handleRegister(c \*gin.Context) {  
    var req RegisterRequest  
    if err := c.ShouldBindJSON(\&req); err \!= nil {  
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})  
        return  
    }

    key := "hash:" \+ req.Hash  
      
    // Use SETNX (Set if Not Exists) to atomically check and set  
    wasSet, err := rdb.SetNX(ctx, key, req.PubKey, 0).Result()  
    if err \!= nil {  
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})  
        return  
    }

    if \!wasSet {  
        // The hash (key) already exists  
        c.JSON(http.StatusConflict, gin.H{"error": "Hash already taken"})  
        return  
    }

    c.JSON(http.StatusCreated, gin.H{"status": "Registered"})  
}

// GET /get\_pub\_key?hash=...  
func handleGetKey(c \*gin.Context) {  
    hash := c.Query("hash")  
    if len(hash) \!= 10 {  
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid hash format"})  
        return  
    }

    key := "hash:" \+ hash  
    pubKey, err := rdb.Get(ctx, key).Result()

    if err \== redis.Nil {  
        c.JSON(http.StatusNotFound, gin.H{"error": "Hash not found"})  
        return  
    } else if err \!= nil {  
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})  
        return  
    }

    c.JSON(http.StatusOK, gin.H{"hash": hash, "pub\_key": pubKey})  
}

// POST /dispose  
func handleDispose(c \*gin.Context) {  
    var req DisposeRequest  
    if err := c.ShouldBindJSON(\&req); err \!= nil {  
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})  
        return  
    }

    key := "hash:" \+ req.Hash

    // 1\. Get the Public Key from Redis  
    pubKeyStr, err := rdb.Get(ctx, key).Result()  
    if err \!= nil {  
        c.JSON(http.StatusNotFound, gin.H{"error": "Hash not found"})  
        return  
    }

    // 2\. \--- CRYPTO-LOGIC (PSEUDOCODE) \---  
    // This is the most complex part. We need to:  
    //    a. Decode the PEM-formatted pubKeyStr into a Go ecdsa.PublicKey  
    //    b. Decode the hex/base64-formatted req.Signature  
    //    c. The "message" that was signed is the hash itself (req.Hash)  
    //    d. Verify(pubKey, sha256(req.Hash), signature)  
      
    // \---\> \[IMPLEMENT VERIFICATION LOGIC HERE\] \<---  
    // Example:  
    // pubKey, \_ := crypto\_utils.ParseECPublicKeyFromPEM(pubKeyStr)  
    // sig, \_ := hex.DecodeString(req.Signature)  
    // msgHash := sha256.Sum256(\[\]byte(req.Hash))  
    // isValid := ecdsa.VerifyASN1(pubKey, msgHash\[:\], sig)  
      
    isValid := true // Placeholder\!

    if \!isValid {  
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})  
        return  
    }

    // 3\. Delete the key  
    if err := rdb.Del(ctx, key).Err(); err \!= nil {  
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})  
        return  
    }

    // 4\. (Optional) Kick user from MQTT  
    // This would require an MQTT management plugin or an API call  
      
    c.JSON(http.StatusOK, gin.H{"status": "Disposed"})  
}

## **3\. Component 2: Mosquitto Broker (mosquitto/)**

* mosquitto.conf:  
  persistence true  
  persistence\_location /mosquitto/data/  
  log\_dest file /mosquitto/log/mosquitto.log

  \# \--- Listeners \---  
  \# We will use WebSockets (MQTTS) for the ESP32  
  listener 1883  
  \# listener 8883 (for MQTTS, requires certs)  
  \# listener 8080 (for MQTTS over WebSockets, requires certs)

  \# \--- Authentication \---  
  \# For simplicity, we can use an ACL file.  
  \# A better solution would be a plugin (e.g., mosquitto-go-auth)  
  \# that checks the Go API's Redis DB.  
  \# For now, we'll allow anonymous (but restricted) access.  
  allow\_anonymous true  
  acl\_file /mosquitto/config/acl.conf

* acl.conf:  
  \# All users can connect.  
  pattern write $SYS/\#  
  pattern read $SYS/\#

  \# Users can only subscribe and publish to their OWN topic.  
  \# This is a limitation. Mosquitto ACLs are hard.  
  \# A "user" here is anonymous.

  \# This allows anyone to publish/subscribe to any message.  
  \# We MUST secure this.  
  pattern write himitsu/messages/\#  
  pattern read himitsu/messages/\#

  \# \--- BETTER ACL (if using username) \---  
  \# If the ESP32 sets its 10-char hash as its MQTT "username":  
  \#  
  \# pattern write himitsu/messages/%u  
  \# pattern read himitsu/messages/%u  
  \#  
  \# This would restrict a user to only their own topic.  
  \# The ESP32 MQTT client \*must\* set its hash as the username.

* Dockerfile:  
  FROM eclipse-mosquitto:latest  
  COPY mosquitto.conf /mosquitto/config/  
  COPY acl.conf /mosquitto/config/

## **4\. Component 3: Docker Compose (docker-compose.yml)**

This file ties all three services together.

version: '3.8'

services:

  api:  
    build:  
      context: ./api  
    ports:  
      \- "8080:8080"  
    volumes:  
      \- ./api:/app  
    depends\_on:  
      \- redis  
    environment:  
      \# Gin run mode  
      \- GIN\_MODE=release   
      \# Redis address (using Docker service name)  
      \- REDIS\_ADDR=redis:6379

  redis:  
    image: redis:7-alpine  
    ports:  
      \- "6379:6379"  
    volumes:  
      \- redis\_data:/data

  mosquitto:  
    build:  
      context: ./mosquitto  
    ports:  
      \- "1883:1883"  
      \# \- "8883:8883" \# Uncomment for MQTTS  
    volumes:  
      \- mosquitto\_data:/mosquitto/data  
      \- mosquitto\_log:/mosquitto/log

volumes:  
  redis\_data:  
  mosquitto\_data:  
  mosquitto\_log:  
