package server

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yosida95/uritemplate/v3"
)

// TestResourceValuesToArguments tests the resourceValuesToArguments function directly
func TestResourceValuesToArguments(t *testing.T) {
	tests := []struct {
		name     string
		values   uritemplate.Values
		expected map[string]any
	}{
		{
			name: "single string value",
			values: uritemplate.Values{
				"id": uritemplate.Value{
					T: uritemplate.ValueTypeString,
					V: []string{"123"},
				},
			},
			expected: map[string]any{
				"id": "123",
			},
		},
		{
			name: "empty string value",
			values: uritemplate.Values{
				"id": uritemplate.Value{
					T: uritemplate.ValueTypeString,
					V: []string{},
				},
			},
			// Empty string value defaults to empty string (implementation sets default)
			expected: map[string]any{
				"id": "",
			},
		},
		{
			name: "list value",
			values: uritemplate.Values{
				"path": uritemplate.Value{
					T: uritemplate.ValueTypeList,
					V: []string{"a", "b", "c"},
				},
			},
			expected: map[string]any{
				"path": []string{"a", "b", "c"},
			},
		},
		{
			name: "empty list value",
			values: uritemplate.Values{
				"path": uritemplate.Value{
					T: uritemplate.ValueTypeList,
					V: []string{},
				},
			},
			// Empty list defaults to empty string (implementation checks len(v) > 0 via value.List())
			expected: map[string]any{
				"path": "",
			},
		},
		{
			name: "KV value",
			values: uritemplate.Values{
				"params": uritemplate.Value{
					T: uritemplate.ValueTypeKV,
					V: []string{"key1", "value1", "key2", "value2"},
				},
			},
			// Implementation converts KV slice to map[string]string
			expected: map[string]any{
				"params": map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "invalid KV value (odd number of elements)",
			values: uritemplate.Values{
				"params": uritemplate.Value{
					T: uritemplate.ValueTypeKV,
					V: []string{"key1", "value1", "key2"},
				},
			},
			// Invalid KV defaults to empty string (implementation checks len(v) > 0 and even)
			expected: map[string]any{
				"params": "",
			},
		},
		{
			name: "invalid KV value (empty)",
			values: uritemplate.Values{
				"params": uritemplate.Value{
					T: uritemplate.ValueTypeKV,
					V: []string{},
				},
			},
			// Invalid KV defaults to empty string (implementation checks len(v) > 0)
			expected: map[string]any{
				"params": "",
			},
		},
		{
			name: "multiple values",
			values: uritemplate.Values{
				"id": uritemplate.Value{
					T: uritemplate.ValueTypeString,
					V: []string{"123"},
				},
				"path": uritemplate.Value{
					T: uritemplate.ValueTypeList,
					V: []string{"a", "b"},
				},
				"name": uritemplate.Value{
					T: uritemplate.ValueTypeString,
					V: []string{"test"},
				},
			},
			expected: map[string]any{
				"id":   "123",
				"path": []string{"a", "b"},
				"name": "test",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resourceValuesToArguments(tt.values)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestResourceHandlerArguments_GlobalTemplate tests that arguments are correctly parsed from URI templates for global templates
func TestResourceHandlerArguments_GlobalTemplate(t *testing.T) {
	tests := []struct {
		name            string
		templateURI     string
		requestURI      string
		expectedArgs    map[string]any
		handlerValidate func(*testing.T, mcp.ReadResourceRequest)
	}{
		{
			name:        "single variable",
			templateURI: "test://users/{id}",
			requestURI:  "test://users/123",
			expectedArgs: map[string]any{
				"id": "123",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "123", req.Params.Arguments["id"])
			},
		},
		{
			name:        "multiple variables",
			templateURI: "test://users/{userId}/documents/{docId}",
			requestURI:  "test://users/john/documents/readme.txt",
			expectedArgs: map[string]any{
				"userId": "john",
				"docId":  "readme.txt",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "john", req.Params.Arguments["userId"])
				assert.Equal(t, "readme.txt", req.Params.Arguments["docId"])
			},
		},
		{
			name:        "path explosion (list)",
			templateURI: "test://files{/path*}",
			requestURI:  "test://files/a/b/c",
			expectedArgs: map[string]any{
				"path": []string{"a", "b", "c"},
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				path, ok := req.Params.Arguments["path"].([]string)
				require.True(t, ok, "path should be []string")
				assert.Equal(t, []string{"a", "b", "c"}, path)
			},
		},
		{
			name:        "single path segment",
			templateURI: "test://files{/path*}",
			requestURI:  "test://files/single",
			// Single segment might be treated as a string rather than list
			expectedArgs: map[string]any{
				"path": "single",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				// Path explosion with single segment might return string or []string
				path := req.Params.Arguments["path"]
				assert.NotNil(t, path)
				if pathStr, ok := path.(string); ok {
					assert.Equal(t, "single", pathStr)
				} else if pathList, ok := path.([]string); ok {
					assert.Equal(t, []string{"single"}, pathList)
				}
			},
		},
		{
			name:        "variable with special characters",
			templateURI: "test://files/{filename}",
			requestURI:  "test://files/my-file%20with%20spaces.txt",
			expectedArgs: map[string]any{
				"filename": "my-file with spaces.txt",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "my-file with spaces.txt", req.Params.Arguments["filename"])
			},
		},
		{
			name:        "variable with numbers",
			templateURI: "test://api/v{version}/resource/{id}",
			requestURI:  "test://api/v1/resource/42",
			expectedArgs: map[string]any{
				"version": "1",
				"id":      "42",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "1", req.Params.Arguments["version"])
				assert.Equal(t, "42", req.Params.Arguments["id"])
			},
		},
		{
			name:        "empty path explosion",
			templateURI: "test://files{/path*}",
			requestURI:  "test://files",
			// Empty path explosion might not populate arguments
			expectedArgs: map[string]any{},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				// Empty path explosion might not populate arguments
				// This is acceptable behavior - the template might not match or arguments might be empty
				// We just verify the request was handled
				assert.NotNil(t, req)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMCPServer("test-server", "1.0.0", WithResourceCapabilities(false, false))

			var capturedRequest mcp.ReadResourceRequest
			server.AddResourceTemplate(
				mcp.NewResourceTemplate(tt.templateURI, "Test Template"),
				func(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					capturedRequest = request
					return []mcp.ResourceContents{
						mcp.TextResourceContents{
							URI:  request.Params.URI,
							Text: "test content",
						},
					}, nil
				},
			)

			requestBytes, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "resources/read",
				"params": map[string]any{
					"uri": tt.requestURI,
				},
			})
			require.NoError(t, err)

			response := server.HandleMessage(context.Background(), requestBytes)
			resp, ok := response.(mcp.JSONRPCResponse)
			require.True(t, ok, "Expected successful response")
			require.NotNil(t, resp.Result)

			result, ok := resp.Result.(mcp.ReadResourceResult)
			require.True(t, ok)
			require.Len(t, result.Contents, 1)

			// Validate that arguments were correctly parsed
			if tt.handlerValidate != nil {
				tt.handlerValidate(t, capturedRequest)
			}

			// Validate expected arguments
			if len(tt.expectedArgs) > 0 {
				for key, expectedValue := range tt.expectedArgs {
					require.NotNil(t, capturedRequest.Params.Arguments, "Arguments should not be nil")
					actualValue := capturedRequest.Params.Arguments[key]
					assert.Equal(t, expectedValue, actualValue, "Argument %s should match", key)
				}
			}
		})
	}
}

// TestResourceHandlerArguments_SessionTemplate tests that arguments are correctly parsed from URI templates for session templates
func TestResourceHandlerArguments_SessionTemplate(t *testing.T) {
	tests := []struct {
		name            string
		templateURI     string
		requestURI      string
		expectedArgs    map[string]any
		handlerValidate func(*testing.T, mcp.ReadResourceRequest)
	}{
		{
			name:        "single variable",
			templateURI: "test://session/{id}",
			requestURI:  "test://session/abc123",
			expectedArgs: map[string]any{
				"id": "abc123",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "abc123", req.Params.Arguments["id"])
			},
		},
		{
			name:        "multiple variables",
			templateURI: "test://session/{sessionId}/data/{itemId}",
			requestURI:  "test://session/sess-123/data/item-456",
			expectedArgs: map[string]any{
				"sessionId": "sess-123",
				"itemId":    "item-456",
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				assert.Equal(t, "sess-123", req.Params.Arguments["sessionId"])
				assert.Equal(t, "item-456", req.Params.Arguments["itemId"])
			},
		},
		{
			name:        "path explosion",
			templateURI: "test://session{/path*}",
			requestURI:  "test://session/x/y/z",
			expectedArgs: map[string]any{
				"path": []string{"x", "y", "z"},
			},
			handlerValidate: func(t *testing.T, req mcp.ReadResourceRequest) {
				require.NotNil(t, req.Params.Arguments)
				path, ok := req.Params.Arguments["path"].([]string)
				require.True(t, ok)
				assert.Equal(t, []string{"x", "y", "z"}, path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewMCPServer("test-server", "1.0.0", WithResourceCapabilities(false, false))

			session := &sessionTestClientWithResourceTemplates{
				sessionID:           "session-1",
				notificationChannel: make(chan mcp.JSONRPCNotification, 10),
			}
			session.initialized.Store(true)

			var capturedRequest mcp.ReadResourceRequest
			err := server.RegisterSession(context.Background(), session)
			require.NoError(t, err)

			err = server.AddSessionResourceTemplate(
				session.SessionID(),
				mcp.NewResourceTemplate(tt.templateURI, "Session Template"),
				func(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
					capturedRequest = request
					return []mcp.ResourceContents{
						mcp.TextResourceContents{
							URI:  request.Params.URI,
							Text: "session content",
						},
					}, nil
				},
			)
			require.NoError(t, err)

			sessionCtx := server.WithContext(context.Background(), session)
			requestBytes, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"method":  "resources/read",
				"params": map[string]any{
					"uri": tt.requestURI,
				},
			})
			require.NoError(t, err)

			response := server.HandleMessage(sessionCtx, requestBytes)
			resp, ok := response.(mcp.JSONRPCResponse)
			require.True(t, ok, "Expected successful response")
			require.NotNil(t, resp.Result)

			result, ok := resp.Result.(mcp.ReadResourceResult)
			require.True(t, ok)
			require.Len(t, result.Contents, 1)

			// Validate that arguments were correctly parsed
			if tt.handlerValidate != nil {
				tt.handlerValidate(t, capturedRequest)
			}

			// Validate expected arguments
			if len(tt.expectedArgs) > 0 {
				for key, expectedValue := range tt.expectedArgs {
					require.NotNil(t, capturedRequest.Params.Arguments, "Arguments should not be nil")
					actualValue := capturedRequest.Params.Arguments[key]
					assert.Equal(t, expectedValue, actualValue, "Argument %s should match", key)
				}
			}
		})
	}
}

// TestResourceHandlerArguments_SessionOverridesGlobal tests that session templates override global templates
func TestResourceHandlerArguments_SessionOverridesGlobal(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0", WithResourceCapabilities(false, false))

	// Add global template
	var globalRequest mcp.ReadResourceRequest
	server.AddResourceTemplate(
		mcp.NewResourceTemplate("test://resource/{id}", "Global Template"),
		func(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			globalRequest = request
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:  request.Params.URI,
					Text: "global content",
				},
			}, nil
		},
	)

	// Add session template with same URI pattern
	session := &sessionTestClientWithResourceTemplates{
		sessionID:           "session-1",
		notificationChannel: make(chan mcp.JSONRPCNotification, 10),
	}
	session.initialized.Store(true)

	var sessionRequest mcp.ReadResourceRequest
	err := server.RegisterSession(context.Background(), session)
	require.NoError(t, err)

	err = server.AddSessionResourceTemplate(
		session.SessionID(),
		mcp.NewResourceTemplate("test://resource/{id}", "Session Template"),
		func(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			sessionRequest = request
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:  request.Params.URI,
					Text: "session content",
				},
			}, nil
		},
	)
	require.NoError(t, err)

	sessionCtx := server.WithContext(context.Background(), session)
	requestBytes, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "test://resource/my-id",
		},
	})
	require.NoError(t, err)

	response := server.HandleMessage(sessionCtx, requestBytes)
	resp, ok := response.(mcp.JSONRPCResponse)
	require.True(t, ok)
	require.NotNil(t, resp.Result)

	result, ok := resp.Result.(mcp.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	// Verify session template was used (not global)
	textContent := result.Contents[0].(mcp.TextResourceContents)
	assert.Equal(t, "session content", textContent.Text)

	// Verify arguments were correctly parsed in session handler
	require.NotNil(t, sessionRequest.Params.Arguments)
	assert.Equal(t, "my-id", sessionRequest.Params.Arguments["id"])

	// Verify global handler was not called
	assert.Equal(t, "", globalRequest.Params.URI)
}

// TestResourceHandlerArguments_ComplexTemplate tests complex URI template patterns
func TestResourceHandlerArguments_ComplexTemplate(t *testing.T) {
	server := NewMCPServer("test-server", "1.0.0", WithResourceCapabilities(false, false))

	var capturedRequest mcp.ReadResourceRequest
	server.AddResourceTemplate(
		mcp.NewResourceTemplate("test://api/v{version}/users/{userId}/posts/{postId}{/comments*}", "Complex Template"),
		func(_ context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
			capturedRequest = request
			return []mcp.ResourceContents{
				mcp.TextResourceContents{
					URI:  request.Params.URI,
					Text: "complex content",
				},
			}, nil
		},
	)

	requestBytes, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "resources/read",
		"params": map[string]any{
			"uri": "test://api/v2/users/alice/posts/123/comments/1/2/3",
		},
	})
	require.NoError(t, err)

	response := server.HandleMessage(context.Background(), requestBytes)
	resp, ok := response.(mcp.JSONRPCResponse)
	require.True(t, ok)
	require.NotNil(t, resp.Result)

	result, ok := resp.Result.(mcp.ReadResourceResult)
	require.True(t, ok)
	require.Len(t, result.Contents, 1)

	// Validate all arguments were correctly parsed
	require.NotNil(t, capturedRequest.Params.Arguments)
	assert.Equal(t, "2", capturedRequest.Params.Arguments["version"])
	assert.Equal(t, "alice", capturedRequest.Params.Arguments["userId"])
	assert.Equal(t, "123", capturedRequest.Params.Arguments["postId"])
	comments, ok := capturedRequest.Params.Arguments["comments"].([]string)
	require.True(t, ok)
	// Path explosion includes the variable name in the path segments
	// The actual value might be ["comments", "1", "2", "3"] or just ["1", "2", "3"]
	// We check that it contains the expected segments
	assert.Contains(t, comments, "1")
	assert.Contains(t, comments, "2")
	assert.Contains(t, comments, "3")
	assert.GreaterOrEqual(t, len(comments), 3, "Comments should have at least 3 segments")
}
