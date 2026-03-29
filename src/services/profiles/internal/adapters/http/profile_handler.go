package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	bberrors "github.com/osmanozen/oo-commerce/pkg/buildingblocks/errors"
	"github.com/osmanozen/oo-commerce/services/profiles/internal/application/commands"
	"github.com/osmanozen/oo-commerce/services/profiles/internal/application/queries"
)

type ProfileHandler struct {
	getProfile        *queries.GetProfileHandler
	updateProfile     *commands.UpdateProfileHandler
	uploadAvatar      *commands.UploadAvatarHandler
	removeAvatar      *commands.RemoveAvatarHandler
	addAddress        *commands.AddAddressHandler
	updateAddress     *commands.UpdateAddressHandler
	deleteAddress     *commands.DeleteAddressHandler
	setDefaultAddress *commands.SetDefaultAddressHandler
}

func NewProfileHandler(
	getProfile *queries.GetProfileHandler,
	updateProfile *commands.UpdateProfileHandler,
	uploadAvatar *commands.UploadAvatarHandler,
	removeAvatar *commands.RemoveAvatarHandler,
	addAddress *commands.AddAddressHandler,
	updateAddress *commands.UpdateAddressHandler,
	deleteAddress *commands.DeleteAddressHandler,
	setDefaultAddress *commands.SetDefaultAddressHandler,
) *ProfileHandler {
	return &ProfileHandler{
		getProfile:        getProfile,
		updateProfile:     updateProfile,
		uploadAvatar:      uploadAvatar,
		removeAvatar:      removeAvatar,
		addAddress:        addAddress,
		updateAddress:     updateAddress,
		deleteAddress:     deleteAddress,
		setDefaultAddress: setDefaultAddress,
	}
}

func (h *ProfileHandler) RegisterRoutes(r chi.Router) {
	r.Route("/api/profiles/me", func(r chi.Router) {
		r.Get("/", h.HandleGetMyProfile)
		r.Put("/", h.HandleUpdateProfile)
		r.Post("/avatar", h.HandleUploadAvatar)
		r.Delete("/avatar", h.HandleRemoveAvatar)
		r.Post("/addresses", h.HandleAddAddress)
		r.Put("/addresses/{addressId}", h.HandleUpdateAddress)
		r.Delete("/addresses/{addressId}", h.HandleDeleteAddress)
		r.Patch("/addresses/{addressId}/default", h.HandleSetDefaultAddress)
	})
}

func (h *ProfileHandler) HandleGetMyProfile(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	result, err := h.getProfile.Handle(r.Context(), queries.GetProfileQuery{UserID: userID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ProfileHandler) HandleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateProfileCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.UserID = extractUserID(r)
	if cmd.UserID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	if _, err := h.updateProfile.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) HandleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	if err := r.ParseMultipartForm(5 << 20); err != nil {
		writeError(w, bberrors.ValidationError("invalid multipart form"))
		return
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeError(w, bberrors.ValidationError("file is required"))
		return
	}
	_ = file.Close()

	result, err := h.uploadAvatar.Handle(r.Context(), commands.UploadAvatarCommand{
		UserID:      userID,
		FileName:    fileHeader.Filename,
		ContentType: fileHeader.Header.Get("Content-Type"),
		SizeBytes:   fileHeader.Size,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *ProfileHandler) HandleRemoveAvatar(w http.ResponseWriter, r *http.Request) {
	userID := extractUserID(r)
	if userID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	if _, err := h.removeAvatar.Handle(r.Context(), commands.RemoveAvatarCommand{UserID: userID}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) HandleAddAddress(w http.ResponseWriter, r *http.Request) {
	var cmd commands.AddAddressCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.UserID = extractUserID(r)
	if cmd.UserID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}

	addressID, err := h.addAddress.Handle(r.Context(), cmd)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"addressId": addressID})
}

func (h *ProfileHandler) HandleUpdateAddress(w http.ResponseWriter, r *http.Request) {
	var cmd commands.UpdateAddressCommand
	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		writeError(w, bberrors.ValidationError("invalid request body"))
		return
	}
	cmd.UserID = extractUserID(r)
	cmd.AddressID = chi.URLParam(r, "addressId")
	if cmd.UserID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	if _, err := h.updateAddress.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) HandleDeleteAddress(w http.ResponseWriter, r *http.Request) {
	cmd := commands.DeleteAddressCommand{
		UserID:    extractUserID(r),
		AddressID: chi.URLParam(r, "addressId"),
	}
	if cmd.UserID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	if _, err := h.deleteAddress.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *ProfileHandler) HandleSetDefaultAddress(w http.ResponseWriter, r *http.Request) {
	cmd := commands.SetDefaultAddressCommand{
		UserID:    extractUserID(r),
		AddressID: chi.URLParam(r, "addressId"),
	}
	if cmd.UserID == "" {
		writeError(w, bberrors.ValidationError("x-user-id header is required"))
		return
	}
	if _, err := h.setDefaultAddress.Handle(r.Context(), cmd); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func extractUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, err error) {
	status := bberrors.MapToHTTPStatus(err)
	resp := bberrors.ToErrorResponse(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}
