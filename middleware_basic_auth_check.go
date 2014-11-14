package main

import "net/http"

import (
	"encoding/base64"
	"errors"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/context"
	"strings"
)

// BasicAuthKeyIsValid uses a username instead of
type BasicAuthKeyIsValid struct {
	TykMiddleware
}

// New lets you do any initialisations for the object can be done here
func (k *BasicAuthKeyIsValid) New() {}

// GetConfig retrieves the configuration from the API config - we user mapstructure for this for simplicity
func (k *BasicAuthKeyIsValid) GetConfig() (interface{}, error) {
	return nil, nil
}

// ProcessRequest will run any checks on the request on the way through the system, return an error to have the chain fail
func (k *BasicAuthKeyIsValid) ProcessRequest(w http.ResponseWriter, r *http.Request, configuration interface{}) (error, int) {
	authHeaderValue := r.Header.Get("Authorization")
	if authHeaderValue == "" {
		// No header value, fail
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
		}).Info("Attempted access with malformed header, no auth header found.")

		authReply := "Basic realm=\"" + k.TykMiddleware.Spec.Name + "\""

		w.Header().Add("WWW-Authenticate", authReply)
		return errors.New("Authorization field missing"), 401
	}

	bits := strings.Split(authHeaderValue, " ")
	if len(bits) != 2 {
		// Header malformed
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
		}).Info("Attempted access with malformed header, header not in basic auth format.")

		return errors.New("Attempted access with malformed header, header not in basic auth format"), 400
	}

	// Decode the username:password string
	authvaluesStr, err := base64.StdEncoding.DecodeString(bits[1])
	if err != nil {
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
		}).Info("Base64 Decoding failed of basic auth data: ", err)

		return errors.New("Attempted access with malformed header, auth data not encoded correctly"), 400
	}

	authValues := strings.Split(string(authvaluesStr), ":")
	if len(authValues) != 2 {
		// Header malformed
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
		}).Info("Attempted access with malformed header, values not in basic auth format.")

		return errors.New("Attempted access with malformed header, values not in basic auth format"), 400
	}

	// Check if API key valid
	keyName := k.TykMiddleware.Spec.OrgID + authValues[0]
	thisSessionState, keyExists := k.TykMiddleware.CheckSessionAndIdentityForValidKey(keyName)
	if !keyExists {
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
			"key":    keyName,
		}).Info("Attempted access with non-existent user.")

		// Fire Authfailed Event
		AuthFailed(k.TykMiddleware, r, authHeaderValue)


		return errors.New("User not authorised"), 403
	}

	// Ensure that the username and password match up
	if thisSessionState.BasicAuthData.Password != authValues[1] {
		log.WithFields(logrus.Fields{
			"path":   r.URL.Path,
			"origin": r.RemoteAddr,
			"key":    keyName,
		}).Info("Attempted access with existing user but failed password check.")

		// Fire Authfailed Event
		AuthFailed(k.TykMiddleware, r, authHeaderValue)

		return errors.New("User not authorised"), 403
	}

	// Set session state on context, we will need it later
	context.Set(r, SessionData, thisSessionState)
	context.Set(r, AuthHeaderValue, keyName)

	// Request is valid, carry on
	return nil, 200
}