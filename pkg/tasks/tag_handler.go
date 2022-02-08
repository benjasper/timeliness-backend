package tasks

import (
	"encoding/json"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/timeliness-app/timeliness-backend/pkg/auth"
	"github.com/timeliness-app/timeliness-backend/pkg/communication"
	"github.com/timeliness-app/timeliness-backend/pkg/logger"
	"github.com/timeliness-app/timeliness-backend/pkg/users"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"math"
	"net/http"
	"strconv"
	"time"
)

// TagHandler handles all tag related API calls
type TagHandler struct {
	TagRepository   TagRepository
	UserRepository  users.UserRepositoryInterface
	TaskRepository  TaskRepositoryInterface
	Logger          logger.Interface
	ResponseManager *communication.ResponseManager
}

// TagAdd is the route for adding a task
func (handler *TagHandler) TagAdd(writer http.ResponseWriter, request *http.Request) {
	tag := Tag{}

	err := json.NewDecoder(request.Body).Decode(&tag)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	userID, err := primitive.ObjectIDFromHex(request.Context().Value(auth.KeyUserID).(string))
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"UserID malformed", err)
		return
	}

	tag.UserID = userID

	v := validator.New()
	err = v.Struct(tag)
	if err != nil {
		for _, e := range err.(validator.ValidationErrors) {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, e.Error(), e)
			return
		}
	}

	existingTag, err := handler.TagRepository.FindByValue(request.Context(), tag.Value, userID.Hex(), false)
	if err == nil && existingTag != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusConflict,
			"This tag already exists", fmt.Errorf("tag already exists"))
		return
	}

	err = handler.TagRepository.Add(request.Context(), &tag)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Persisting tag in database did not work", err)
		return
	}

	handler.ResponseManager.Respond(writer, &tag)
}

// TagUpdate is the route for updating a Tag
func (handler *TagHandler) TagUpdate(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	tagID := mux.Vars(request)["tagID"]

	tag, err := handler.TagRepository.FindByID(request.Context(), tagID, userID, false)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't find tag", err)
		return
	}

	tagUpdate := (*TagUpdate)(tag)

	err = json.NewDecoder(request.Body).Decode(&tagUpdate)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong format", err)
		return
	}

	err = handler.TagRepository.Update(request.Context(), (*Tag)(tagUpdate))
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusNotFound, "Couldn't update tag", err)
		return
	}

	handler.ResponseManager.Respond(writer, (*Tag)(tagUpdate))
}

// TagDelete deletes a tag
func (handler *TagHandler) TagDelete(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)
	tagID := mux.Vars(request)["tagID"]

	err := handler.TagRepository.Delete(request.Context(), tagID, userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not delete tag", err)
		return
	}

	err = handler.TaskRepository.DeleteTag(request.Context(), tagID, userID)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError,
			"Could not delete tag(s) from task(s)", err)
		return
	}

	writer.WriteHeader(http.StatusNoContent)
}

// GetAllTags is the route for getting all tags
func (handler *TagHandler) GetAllTags(writer http.ResponseWriter, request *http.Request) {
	userID := request.Context().Value(auth.KeyUserID).(string)

	var page = 0
	var pageSize = 10
	var err error

	queryPage := request.URL.Query().Get("page")
	queryPageSize := request.URL.Query().Get("pageSize")
	lastModifiedAt := request.URL.Query().Get("lastModifiedAt")
	includeDeletedQuery := request.URL.Query().Get("includeDeleted")

	includeDeleted := false
	if includeDeletedQuery != "" {
		includeDeleted, err = strconv.ParseBool(includeDeletedQuery)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad value for includeDeleted", err)
			return
		}
	}

	if queryPage != "" {
		page, err = strconv.Atoi(queryPage)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter page", err)
			return
		}
	}

	if queryPageSize != "" {
		pageSize, err = strconv.Atoi(queryPageSize)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Bad query parameter pageSize", err)
			return
		}

		if pageSize > 25 {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest,
				"Page size can't be more than 25", nil)
			return
		}
	}

	var filters []Filter

	if lastModifiedAt != "" {
		timeValue, err := time.Parse(time.RFC3339, lastModifiedAt)
		if err != nil {
			handler.ResponseManager.RespondWithError(writer, http.StatusBadRequest, "Wrong date format in query string", err)
			return
		}
		filters = append(filters, Filter{Field: "lastModifiedAt", Operator: "$gte", Value: timeValue})
	}

	tags, count, err := handler.TagRepository.FindAll(request.Context(), userID, page, pageSize, filters, includeDeleted)
	if err != nil {
		handler.ResponseManager.RespondWithError(writer, http.StatusInternalServerError, "Error in query", err)
		return
	}

	pages := float64(count) / float64(pageSize)

	var response = map[string]interface{}{
		"results": tags,
		"pagination": map[string]interface{}{
			"resultCount": count,
			"pageSize":    pageSize,
			"pageIndex":   page,
			"pages":       int(math.Ceil(pages)),
		},
	}

	handler.ResponseManager.Respond(writer, response)
}
