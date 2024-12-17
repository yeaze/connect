// Copyright 2024 Redpanda Data, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package serverless_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yeaze/connect/v4/internal/serverless"

	_ "github.com/yeaze/connect/v4/public/components/pure"
)

func TestServerlessHandlerDefaults(t *testing.T) {
	h, err := serverless.NewHandler(`
pipeline:
  processors:
    - mapping: 'root = content().uppercase()'
logger:
  level: NONE
`)
	require.NoError(t, err)

	ctx, done := context.WithTimeout(context.Background(), time.Second*5)
	defer done()

	res, err := h.Handle(ctx, "hello world")
	require.NoError(t, err)

	assert.Equal(t, "HELLO WORLD", res)

	require.NoError(t, h.Close(ctx))
}
