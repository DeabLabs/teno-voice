package tiktoken

import (
	"fmt"
	"strings"

	"github.com/disgoorg/log"
	"github.com/pkoukk/tiktoken-go"
)

func TokenCount(text string, model string) int {
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		log.Errorf("getEncoding: %v", err)
		return 0
	}

	// encode
	token := tkm.Encode(text, nil, nil)

	// num_tokens
	return len(token)
}

func TruncateString(text string, limit int, model string) (string, error) {
	// get the tokenizer model
	tkm, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return "", fmt.Errorf("getEncoding: %v", err)
	}

	// encode the string into tokens
	tokens := tkm.Encode(text, nil, nil)

	// if the token count is within the limit, return the original string
	if len(tokens) <= limit {
		return text, nil
	}

	// find the index where to start the slice from
	startIndex := len(tokens) - limit

	// truncate the tokens
	truncatedTokens := tokens[startIndex:]

	// decode the truncated tokens back to a string
	truncatedString := tkm.Decode(truncatedTokens)
	if truncatedString == "" {
		return "", fmt.Errorf("error decoding tokens")
	}

	return strings.TrimSpace(truncatedString), nil
}
