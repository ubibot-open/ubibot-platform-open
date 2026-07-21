package api

import "github.com/gin-gonic/gin"

// NewRouter wires the three device-facing endpoints onto a gin engine.
func NewRouter(s *Server) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	v1.POST("/auth/time", s.TimeSync)
	v1.POST("/auth/activate", s.Activate)
	v1.POST("/data/report", s.Report)

	return r
}
