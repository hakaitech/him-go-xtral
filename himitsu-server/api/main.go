package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()
var rdb *redis.Client

// --- Pydantic-like Models ---
type RegisterRequest struct {
	Hash   string `json:"hash" binding:"required,len=10"`
	PubKey string `json:"pub_key" binding:"required"`
}

type DisposeRequest struct {
	Hash      string `json:"hash" binding:"required,len=10"`
	Signature string `json:"signature" binding:"required"`
}

// --- Main Setup ---
func main() {
	// Get Redis address from environment variable, default to redis:6379
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdb = redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	router := gin.Default()

	router.POST("/register", handleRegister)
	router.GET("/get_pub_key", handleGetKey)
	router.POST("/dispose", handleDispose)

	log.Fatal(router.Run(":8080"))
}

// --- Endpoint Handlers ---

// POST /register
func handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := "hash:" + req.Hash

	// Use SETNX (Set if Not Exists) to atomically check and set
	wasSet, err := rdb.SetNX(ctx, key, req.PubKey, 0).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	if !wasSet {
		// The hash (key) already exists
		c.JSON(http.StatusConflict, gin.H{"error": "Hash already taken"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "Registered"})
}

// GET /get_pub_key?hash=...
func handleGetKey(c *gin.Context) {
	hash := c.Query("hash")
	if len(hash) != 10 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid hash format"})
		return
	}

	key := "hash:" + hash
	pubKey, err := rdb.Get(ctx, key).Result()

	if err == redis.Nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hash not found"})
		return
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"hash": hash, "pub_key": pubKey})
}

// POST /dispose
func handleDispose(c *gin.Context) {
	var req DisposeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key := "hash:" + req.Hash

	// 1. Get the Public Key from Redis
	pubKeyStr, err := rdb.Get(ctx, key).Result()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Hash not found"})
		return
	}

	// 2. --- CRYPTO-LOGIC ---
	// Parse the PEM-formatted public key
	pubKey, err := parseECPublicKeyFromPEM(pubKeyStr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid public key format"})
		return
	}

	// Decode the hex-formatted signature
	sig, err := hex.DecodeString(req.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature format"})
		return
	}

	// The message that was signed is the hash itself
	msgHash := sha256.Sum256([]byte(req.Hash))

	// Verify the signature
	isValid := ecdsa.VerifyASN1(pubKey, msgHash[:], sig)

	if !isValid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// 3. Delete the key
	if err := rdb.Del(ctx, key).Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 4. (Optional) Kick user from MQTT
	// This would require an MQTT management plugin or an API call

	c.JSON(http.StatusOK, gin.H{"status": "Disposed"})
}

// --- Crypto Utility Functions ---

// parseECPublicKeyFromPEM decodes a PEM-formatted EC public key
func parseECPublicKeyFromPEM(pubKeyPEM string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		return nil, x509.ErrUnsupportedAlgorithm
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	ecdsaPubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, x509.ErrUnsupportedAlgorithm
	}

	return ecdsaPubKey, nil
}
