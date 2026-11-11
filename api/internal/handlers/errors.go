package handlers

import (
	"log"
	"net/http"
)

//Metadata are stored by namespace, so they can't be given without one
func writeMissingNamespace(writer http.ResponseWriter) {
	log.Printf("Metadatas given without namespace")
	http.Error(writer, "Metadata requires a namespace, given by the namespace query param or the Namespace header", http.StatusBadRequest)
}
