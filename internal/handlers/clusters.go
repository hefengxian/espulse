package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hefengxian/espulse/internal/database"
	"github.com/hefengxian/espulse/internal/models"
)

// ListClusters returns all clusters
func ListClusters(c *gin.Context) {
	rows, err := database.DB.Query("SELECT id, name, hosts, auth_type, username, color, notes, created_at, updated_at FROM clusters ORDER BY created_at DESC")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	clusters := []models.Cluster{}
	for rows.Next() {
		var cluster models.Cluster
		err := rows.Scan(
			&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
			&cluster.Username, &cluster.Color, &cluster.Notes,
			&cluster.CreatedAt, &cluster.UpdatedAt,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		clusters = append(clusters, cluster)
	}

	c.JSON(http.StatusOK, clusters)
}

// CreateCluster adds a new cluster
func CreateCluster(c *gin.Context) {
	var cluster models.Cluster
	if err := c.ShouldBindJSON(&cluster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cluster.ID = uuid.New().String()
	cluster.CreatedAt = time.Now()
	cluster.UpdatedAt = time.Now()

	_, err := database.DB.Exec(
		"INSERT INTO clusters (id, name, hosts, auth_type, username, password, api_key, color, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		cluster.ID, cluster.Name, cluster.Hosts, cluster.AuthType,
		cluster.Username, cluster.Password, cluster.APIKey,
		cluster.Color, cluster.Notes, cluster.CreatedAt, cluster.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cluster)
}

// GetCluster returns a single cluster
func GetCluster(c *gin.Context) {
	id := c.Param("id")
	var cluster models.Cluster
	err := database.DB.QueryRow(
		"SELECT id, name, hosts, auth_type, username, color, notes, created_at, updated_at FROM clusters WHERE id = ?",
		id,
	).Scan(
		&cluster.ID, &cluster.Name, &cluster.Hosts, &cluster.AuthType,
		&cluster.Username, &cluster.Color, &cluster.Notes,
		&cluster.CreatedAt, &cluster.UpdatedAt,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cluster not found"})
		return
	}

	c.JSON(http.StatusOK, cluster)
}

// DeleteCluster removes a cluster
func DeleteCluster(c *gin.Context) {
	id := c.Param("id")
	_, err := database.DB.Exec("DELETE FROM clusters WHERE id = ?", id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cluster deleted"})
}
