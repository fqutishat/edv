/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package operation

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"github.com/trustbloc/edge-core/pkg/storage/memstore"

	"github.com/stretchr/testify/require"
)

const (
	testVaultID = "urn:uuid:abc5a436-21f9-4b4c-857d-1f5569b2600d"

	testDataVaultConfigurationWithBlankReferenceID = `{
  "sequence": 0,
  "controller": "did:example:123456789",
  "referenceId": "",
  "kek": {
    "id": "https://example.com/kms/12345",
    "type": "AesKeyWrappingKey2019"
  },
  "hmac": {
    "id": "https://example.com/kms/67891",
    "type": "Sha256HmacKey2019"
  }
}`

	testDataVaultConfiguration = `{
  "sequence": 0,
  "controller": "did:example:123456789",
  "referenceId": "` + testVaultID + `",
  "kek": {
    "id": "https://example.com/kms/12345",
    "type": "AesKeyWrappingKey2019"
  },
  "hmac": {
    "id": "https://example.com/kms/67891",
    "type": "Sha256HmacKey2019"
  }
}`

	testDocID = "VJYHHJx4C8J9Fsgz7rZqSp"

	testEncryptedDocument = `{"id":"` + testDocID + `","sequence":0,"jwe":{"protected":"eyJlbmMiOiJjaGFjaGEyMHBvbHkxM` +
		`zA1X2lldGYiLCJ0eXAiOiJKV00vMS4wIiwiYWxnIjoiQXV0aGNyeXB0IiwicmVjaXBpZW50cyI6W3siZW5jcnlwdGVkX2tleSI6ImdLcXNYN` +
		`m1HUXYtS3oyelQzMndIbE5DUjFiVU54ZlRTd0ZYcFVWb3FIMjctQUN0bURpZHBQdlVRcEdKSDZqMDkiLCJoZWFkZXIiOnsia2lkIjoiNzd6e` +
		`WlNeHY0SlRzc2tMeFdFOWI1cVlDN2o1b3Fxc1VMUnFhcVNqd1oya1kiLCJzZW5kZXIiOiJiNmhrRkpXM2RfNmZZVjAtcjV0WEJoWnBVVmtrY` +
		`XhBSFBDUEZxUDVyTHh3aGpwdFJraTRURjBmTEFNcy1seWd0Ym9PQmtnUDhWNWlwaDdndEVNcTAycmFDTEstQm5GRWo3dWk5Rmo5NkRleFRlR` +
		`zl6OGdab1lveXY5ZE09IiwiaXYiOiJjNHMzdzBlRzhyZGhnaC1EZnNjOW5Cb3BYVHA1OEhNZiJ9fV19","iv":"e8mXGCAamvwYcdf2",` +
		`"ciphertext":"dLKWmjFyL-G1uqF588Ya0g10QModI-q0f7vw_v3_jhzskuNqX7Yx4aSD7x2jhUdat82kHS4qLYw8BuUGvGimI_sCQ9m3On` +
		`QTHSjZnpg7VWRqAULBC3MSTtBa1DtZjZL4C0Y=","tag":"W4yJzyuGYzuZtZMRv2bDUg=="}}`

	// All of the characters in the ID below are NOT in the base58 alphabet, so this ID is not base58 encoded
	testEncryptedDocumentWithNonBase58ID = `{
  "id": "0OIl"
}`

	testEncryptedDocumentWithIDThatWasNot128BitsBeforeBase58Encoding = `{
  "id": "2CHi6"
}`
	testPrefix = "testPrefix"
)

func TestCreateDataVaultHandler_InvalidDataVaultConfigurationJSON(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createVaultHandler := getHandler(t, op, createVaultEndpoint)

	req, err := http.NewRequest(http.MethodPost, "", bytes.NewBuffer([]byte("")))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	createVaultHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "EOF")
}

func TestCreateDataVaultHandler_DataVaultConfigurationWithBlankReferenceIDJSON(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	req, err := http.NewRequest(http.MethodPost, "",
		bytes.NewBuffer([]byte(testDataVaultConfigurationWithBlankReferenceID)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	createVaultEndpointHandler := getHandler(t, op, createVaultEndpoint)
	createVaultEndpointHandler.Handle().ServeHTTP(rr, req)

	resp, err := ioutil.ReadAll(rr.Body)
	require.NoError(t, err)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, "referenceId can't be blank", string(resp))
}

func TestCreateDataVaultHandler_ValidDataVaultConfigurationJSON(t *testing.T) {
	t.Run("Without prefix", func(t *testing.T) {
		op := New(memstore.NewProvider(), "")

		createDataVaultExpectSuccess(t, op)
	})
	t.Run("With prefix", func(t *testing.T) {
		op := New(memstore.NewProvider(), testPrefix)

		createDataVaultExpectSuccess(t, op)
	})
}

type failingResponseWriter struct {
}

func (f failingResponseWriter) Header() http.Header {
	return nil
}

func (f failingResponseWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("failingResponseWriter always fails")
}

func (f failingResponseWriter) WriteHeader(statusCode int) {
}

type failingReadCloser struct{}

func (m failingReadCloser) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("failingReadCloser always fails")
}

func (m failingReadCloser) Close() error {
	return nil
}

type alwaysReturnBarebonesDataVaultConfigurationReadCloser struct{}

func (a alwaysReturnBarebonesDataVaultConfigurationReadCloser) Read(p []byte) (n int, err error) {
	dataVaultConfigBytes := []byte(`{
  "referenceId": "` + testVaultID + `"
}`)

	_ = copy(p, dataVaultConfigBytes)

	return 68, io.EOF
}

func (a alwaysReturnBarebonesDataVaultConfigurationReadCloser) Close() error {
	return nil
}

type alwaysReturnBarebonesEncryptedDocumentReadCloser struct{}

func (a alwaysReturnBarebonesEncryptedDocumentReadCloser) Read(p []byte) (n int, err error) {
	documentBytes := []byte(`{
  "id": "` + testDocID + `"
}`)

	_ = copy(p, documentBytes)

	return 59, io.EOF
}

func (a alwaysReturnBarebonesEncryptedDocumentReadCloser) Close() error {
	return nil
}

type mockContext struct {
	valueToReturnWhenValueMethodCalled interface{}
}

func (m mockContext) Deadline() (deadline time.Time, ok bool) {
	panic("implement me")
}

func (m mockContext) Done() <-chan struct{} {
	panic("implement me")
}

func (m mockContext) Err() error {
	panic("implement me")
}

func (m mockContext) Value(key interface{}) interface{} {
	return m.valueToReturnWhenValueMethodCalled
}

func TestCreateDataVaultHandler_ResponseWriterFailsWhileWritingDecodeError(t *testing.T) {
	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	op := New(memstore.NewProvider(), "")

	op.createDataVaultHandler(failingResponseWriter{}, &http.Request{Body: failingReadCloser{}})

	require.Contains(t, logContents.String(), "Failed to write response for data vault creation failure"+
		" due to the provided data vault configuration: failingResponseWriter always fails")
}

func TestCreateDataVaultHandler_ResponseWriterFailsWhileWritingCreateDataVaultError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	op.createDataVaultHandler(failingResponseWriter{},
		&http.Request{Body: alwaysReturnBarebonesDataVaultConfigurationReadCloser{}})

	require.Contains(t, logContents.String(), "Failed to write response for data vault creation failure:"+
		" failingResponseWriter always fails")
}

func TestCreateDataVaultHandler_DuplicateDataVault(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(testDataVaultConfiguration)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	createVaultEndpointHandler := getHandler(t, op, createVaultEndpoint)
	createVaultEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code)
	require.Equal(t, fmt.Sprintf("Data vault creation failed: %s", DuplicateVaultErrMsg), rr.Body.String())
}

func TestCreateDocumentHandler_ValidEncryptedDocumentJSON(t *testing.T) {
	t.Run("Without prefix", func(t *testing.T) {
		op := New(memstore.NewProvider(), "")

		createDataVaultExpectSuccess(t, op)

		storeEncryptedDocumentExpectSuccess(t, op)
	})
	t.Run("With prefix", func(t *testing.T) {
		op := New(memstore.NewProvider(), testPrefix)

		createDataVaultExpectSuccess(t, op)

		storeEncryptedDocumentExpectSuccess(t, op)
	})
}

func TestCreateDocumentHandler_InvalidEncryptedDocumentJSON(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte("")))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), "EOF")
}

func TestCreateDocumentHandler_DocIDIsNotBase58Encoded(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(testEncryptedDocumentWithNonBase58ID)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, NotBase58EncodedErrMsg, rr.Body.String())
}

func TestCreateDocumentHandler_DocIDWasNot128BitsBeforeEncodingAsBase58(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	req, err := http.NewRequest("POST", "",
		bytes.NewBuffer([]byte(testEncryptedDocumentWithIDThatWasNot128BitsBeforeBase58Encoding)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Equal(t, Not128BitValueErrMsg, rr.Body.String())
}

func TestCreateDocumentHandler_DuplicateDocuments(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(testEncryptedDocument)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)
	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), DuplicateDocumentErrMsg)
}

func TestCreateDocumentHandler_VaultDoesNotExist(t *testing.T) {
	op := New(memstore.NewProvider(), "")
	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(testEncryptedDocument)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.Contains(t, rr.Body.String(), VaultNotFoundErrMsg)
}

func TestCreateDocumentHandler_UnableToEscape(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	req, err := http.NewRequest("POST", "", bytes.NewBuffer([]byte(testEncryptedDocument)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = "%"

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, "", rr.Header().Get("Location"))
	require.Equal(t, fmt.Sprintf(`unable to escape %s path variable: invalid URL escape "%%"`, vaultIDPathVariable),
		rr.Body.String())
}

func TestCreateDocumentHandler_ResponseWriterFailsWhileWritingDecodeError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	op.createDocumentHandler(failingResponseWriter{}, &http.Request{Body: failingReadCloser{}})

	require.Contains(t, logContents.String(), "Failed to write response for document creation failure:"+
		" failingResponseWriter always fails")
}

func TestCreateDocumentHandler_ResponseWriterFailsWhileWritingUnableToUnescapeVaultIDError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	request := http.Request{Body: alwaysReturnBarebonesEncryptedDocumentReadCloser{}}

	op.createDocumentHandler(failingResponseWriter{},
		request.WithContext(mockContext{valueToReturnWhenValueMethodCalled: getMapWithVaultIDThatCannotBeEscaped()}))

	require.Contains(t, logContents.String(),
		fmt.Sprintf("Failed to write response for %s unescaping failure: failingResponseWriter always fails",
			vaultIDPathVariable))
}

func TestCreateDocumentHandler_ResponseWriterFailsWhileWritingCreateDocumentError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	op.createDocumentHandler(failingResponseWriter{},
		&http.Request{Body: alwaysReturnBarebonesEncryptedDocumentReadCloser{}})

	require.Contains(t, logContents.String(), "Failed to write response for document creation failure:"+
		" failingResponseWriter always fails")
}

func TestReadDocumentHandler_DocumentExists(t *testing.T) {
	t.Run("Without prefix", func(t *testing.T) {
		readDocumentExpectSuccess(t, "")
	})
	t.Run("With prefix", func(t *testing.T) {
		readDocumentExpectSuccess(t, testPrefix)
	})
}

func readDocumentExpectSuccess(t *testing.T, prefix string) {
	op := New(memstore.NewProvider(), prefix)

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	readDocumentEndpointHandler := getHandler(t, op, readDocumentEndpoint)

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID
	urlVars[docIDPathVariable] = testDocID

	req = mux.SetURLVars(req, urlVars)

	readDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	require.Equal(t, testEncryptedDocument, rr.Body.String())
}

func TestReadDocumentHandler_VaultDoesNotExist(t *testing.T) {
	op := New(memstore.NewProvider(), "")
	readDocumentEndpointHandler := getHandler(t, op, readDocumentEndpoint)

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID
	urlVars[docIDPathVariable] = testDocID

	req = mux.SetURLVars(req, urlVars)

	readDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Contains(t, rr.Body.String(), VaultNotFoundErrMsg)
}

func TestReadDocumentHandler_DocumentDoesNotExist(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	readDocumentEndpointHandler := getHandler(t, op, readDocumentEndpoint)

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID
	urlVars[docIDPathVariable] = testDocID

	req = mux.SetURLVars(req, urlVars)

	readDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Contains(t, rr.Body.String(), DocumentNotFoundErrMsg)
}

func TestReadDocumentHandler_UnableToEscapeVaultIDPathVariable(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	readDocumentEndpointHandler := getHandler(t, op, readDocumentEndpoint)

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = "%"
	urlVars[docIDPathVariable] = testDocID

	req = mux.SetURLVars(req, urlVars)

	readDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	require.Equal(t, fmt.Sprintf(`unable to escape %s path variable: invalid URL escape "%%"`, vaultIDPathVariable),
		rr.Body.String())
}

func TestReadDocumentHandler_UnableToEscapeDocumentIDPathVariable(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	readDocumentEndpointHandler := getHandler(t, op, readDocumentEndpoint)

	req, err := http.NewRequest(http.MethodGet, "", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = vaultIDPathVariable
	urlVars[docIDPathVariable] = "%"

	req = mux.SetURLVars(req, urlVars)

	readDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusInternalServerError, rr.Code)

	require.Equal(t, fmt.Sprintf(`unable to escape %s path variable: invalid URL escape "%%"`, docIDPathVariable),
		rr.Body.String())
}

func TestReadDocumentHandler_ResponseWriterFailsWhileWritingUnableToUnescapeVaultIDError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	request := http.Request{}

	op.readDocumentHandler(failingResponseWriter{},
		request.WithContext(mockContext{valueToReturnWhenValueMethodCalled: getMapWithVaultIDThatCannotBeEscaped()}))

	require.Contains(t, logContents.String(),
		fmt.Sprintf("Failed to write response for %s unescaping failure: failingResponseWriter always fails",
			vaultIDPathVariable))
}

func TestReadDocumentHandler_ResponseWriterFailsWhileWritingUnableToUnescapeDocIDError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	request := http.Request{}

	op.readDocumentHandler(failingResponseWriter{},
		request.WithContext(mockContext{valueToReturnWhenValueMethodCalled: getMapWithDocIDThatCannotBeEscaped()}))

	require.Contains(t, logContents.String(),
		fmt.Sprintf("Failed to write response for %s unescaping failure: failingResponseWriter always fails",
			docIDPathVariable))
}

func TestReadDocumentHandler_ResponseWriterFailsWhileWritingReadDocumentError(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	op.readDocumentHandler(failingResponseWriter{}, &http.Request{})

	require.Contains(t, logContents.String(), "Failed to write response for document retrieval failure:"+
		" failingResponseWriter always fails")
}

func TestReadDocumentHandler_ResponseWriterFailsWhileWritingRetrievedDocument(t *testing.T) {
	op := New(memstore.NewProvider(), "")

	createDataVaultExpectSuccess(t, op)

	storeEncryptedDocumentExpectSuccess(t, op)

	var logContents bytes.Buffer

	log.SetOutput(&logContents)

	request := http.Request{}

	op.readDocumentHandler(failingResponseWriter{},
		request.WithContext(mockContext{valueToReturnWhenValueMethodCalled: getMapWithValidVaultIDAndDocID()}))

	require.Contains(t, logContents.String(), "Failed to write response for document retrieval success:"+
		" failingResponseWriter always fails")
}

func TestGetFullStoreName(t *testing.T) {
	t.Run("Without prefix", func(t *testing.T) {
		vc := VaultCollection{dbPrefix: ""}
		storeName := vc.getFullStoreName(testVaultID)
		require.Equal(t, testVaultID, storeName)
	})
	t.Run("With prefix", func(t *testing.T) {
		vc := VaultCollection{dbPrefix: testPrefix}
		storeName := vc.getFullStoreName(testVaultID)
		require.Equal(t, testPrefix+"_"+testVaultID, storeName)
	})
}

func createDataVaultExpectSuccess(t *testing.T, op *Operation) {
	req, err := http.NewRequest(http.MethodPost, "", bytes.NewBuffer([]byte(testDataVaultConfiguration)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	createVaultEndpointHandler := getHandler(t, op, createVaultEndpoint)
	createVaultEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, "/encrypted-data-vaults/"+testVaultID, rr.Header().Get("Location"))
}

func storeEncryptedDocumentExpectSuccess(t *testing.T, op *Operation) {
	req, err := http.NewRequest("POST", "",
		bytes.NewBuffer([]byte(testEncryptedDocument)))
	require.NoError(t, err)

	rr := httptest.NewRecorder()

	urlVars := make(map[string]string)
	urlVars[vaultIDPathVariable] = testVaultID

	req = mux.SetURLVars(req, urlVars)

	createDocumentEndpointHandler := getHandler(t, op, createDocumentEndpoint)

	createDocumentEndpointHandler.Handle().ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code)
	require.Equal(t, "/encrypted-data-vaults/"+testVaultID+"/"+"docs/"+testDocID, rr.Header().Get("Location"))
}

func getHandler(t *testing.T, op *Operation, lookup string) Handler {
	return getHandlerWithError(t, op, lookup)
}

func getHandlerWithError(t *testing.T, op *Operation, lookup string) Handler {
	return handlerLookup(t, op, lookup)
}

func handlerLookup(t *testing.T, op *Operation, lookup string) Handler {
	handlers := op.GetRESTHandlers()
	require.NotEmpty(t, handlers)

	for _, h := range handlers {
		if h.Path() == lookup {
			return h
		}
	}

	require.Fail(t, "unable to find handler")

	return nil
}

func getMapWithValidVaultIDAndDocID() map[string]string {
	return map[string]string{
		"vaultID": testVaultID,
		"docID":   testDocID,
	}
}

func getMapWithVaultIDThatCannotBeEscaped() map[string]string {
	return map[string]string{
		"vaultID": "%",
	}
}

func getMapWithDocIDThatCannotBeEscaped() map[string]string {
	return map[string]string{
		"docID": "%",
	}
}
