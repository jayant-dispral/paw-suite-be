#!/bin/bash

# Script to traverse a directory and print file paths with their content
# Usage: ./list-files.sh <directory>

# Check if directory argument is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <directory>"
    exit 1
fi

dir="$1"

# Check if directory exists
if [ ! -d "$dir" ]; then
    echo "Error: Directory '$dir' does not exist"
    exit 1
fi

# Traverse all files in the directory
find "$dir" -type f | sort | while read -r file; do
    # Print the file path relative to current directory
    echo "=== $file ==="
    # Print file content
    cat "$file"
    echo ""  # Add a blank line between files
done
