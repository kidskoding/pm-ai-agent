# PM AI Agent (Go Edition)

An autonomous Product Management AI agent built in Go that transforms high-level ideas into structured user stories and tasks.

## Project Vision: TUI-First Experience
This application is an interactive Terminal User Interface (TUI) built with the Charm stack. It provides a rich, responsive experience for managing the backlog and interacting with the AI agent.

## Core Technologies
- **Go**: Language of choice for CLI tools and agents.
- **Bubble Tea**: The Elm-inspired framework for building TUIs.
- **Lip Gloss**: For terminal styling and layout.
- **LangChainGo**: For building LLM-powered applications and agent chains.
- **Google Generative AI (via LangChainGo)**: For interacting with Gemini models.
- **Godotenv**: For environment variable management.

## Engineering Standards

### Git Standards
- **Commit Messages**: Append all commits with the following signature:
  `Co-authored-by: gemini-cli ${MODELNAME} <218195315+gemini-cli@users.noreply.github.com>`

### TUI Architecture
- **Model-Update-View (Elm architecture)**: Strictly follow the Bubble Tea pattern.
- **Commands (tea.Cmd)**: All I/O operations (like API calls) must be wrapped in a `tea.Cmd` to maintain the purity of the `Update` function.
- **Viewport Management**: Use the `viewport` bubble for handling long chat histories and scrolling.

### Agent Logic
- **Chat Sessions**: Maintain stateful conversations using the `genai.ChatSession`.
- **Context**: Pass `context.Background()` for lifecycle management of API calls.
- **Environment**: Sensitive keys must be loaded via `.env` and never committed.

### Code Style
- Follow standard Go conventions (`gofmt`, `go vet`).
- Prefer descriptive naming for messages and models.

## Workflows
- **Running**: Use `go run main.go` to start the TUI.
- **Dependencies**: Use `go mod tidy` to manage modules.
- **Testing**: Implement standard Go tests for logic that can be decoupled from the TUI.
