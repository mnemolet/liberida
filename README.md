# liberida

## Exporting Conversations

The export command allows you to save your chat conversations in different formats for sharing, backup, or analysis.

### Commands

#### Export Current Session
```
# Export the most recent session to stdout (default: md format)
liberida export current

# Export to markdown file
liberida export current --format md --output my-conversation

# Export to JSON file
liberida export current --format json --output my-conversation.json
```

#### Export Specific Session
```
# Export session by ID to stdout
liberida export session 5

# Export session 5 to markdown file
liberida export session 5 --format md --output session-5

# Export session 5 to JSON
liberida export session 5 --format json --output session-5.json
```

#### Export All Sessions
```
# Export all sessions to markdown files (creates export-session-1.md, export-session-2.md, etc.)
liberida export all --output export

# Export all sessions to JSON files
liberida export all --format json --output export
```
