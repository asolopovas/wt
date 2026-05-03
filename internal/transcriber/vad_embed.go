package transcriber

import _ "embed"

//go:embed assets/ggml-silero-v6.2.0.bin
var vadModelBytes []byte

const embeddedVADName = "ggml-silero-v6.2.0.bin"
