#!/bin/bash
# Script para auto-incrementar versión basado en Conventional Commits

set -e

# Colores para output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

VERSION_FILE=".github/version.txt"

# Leer versión actual
if [ ! -f "$VERSION_FILE" ]; then
    echo -e "${RED}Error: $VERSION_FILE no existe${NC}"
    exit 1
fi

CURRENT_VERSION=$(cat "$VERSION_FILE")
echo -e "${YELLOW}Versión actual: $CURRENT_VERSION${NC}"

# Parsear versión
IFS='.' read -r -a version_parts <<< "$CURRENT_VERSION"
MAJOR="${version_parts[0]}"
MINOR="${version_parts[1]}"
PATCH="${version_parts[2]}"

# Obtener último tag
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [ -z "$LAST_TAG" ]; then
    echo -e "${YELLOW}No hay tags previos, usando HEAD${NC}"
    COMMIT_RANGE="HEAD"
else
    echo -e "${YELLOW}Último tag: $LAST_TAG${NC}"
    COMMIT_RANGE="$LAST_TAG..HEAD"
fi

# Analizar commits para determinar bump
echo -e "${YELLOW}Analizando commits desde $COMMIT_RANGE${NC}"

HAS_BREAKING=false
HAS_FEAT=false
HAS_FIX=false

while IFS= read -r commit_msg; do
    echo "  - $commit_msg"

    # Detectar BREAKING CHANGE
    if echo "$commit_msg" | grep -q "BREAKING CHANGE"; then
        HAS_BREAKING=true
    fi

    # Detectar feat:
    if echo "$commit_msg" | grep -q "^feat:"; then
        HAS_FEAT=true
    fi

    # Detectar fix:
    if echo "$commit_msg" | grep -q "^fix:"; then
        HAS_FIX=true
    fi
done < <(git log $COMMIT_RANGE --pretty=format:"%s")

# Determinar bump
BUMP_TYPE="none"

if [ "$HAS_BREAKING" = true ]; then
    BUMP_TYPE="major"
    MAJOR=$((MAJOR + 1))
    MINOR=0
    PATCH=0
elif [ "$HAS_FEAT" = true ]; then
    BUMP_TYPE="minor"
    MINOR=$((MINOR + 1))
    PATCH=0
elif [ "$HAS_FIX" = true ]; then
    BUMP_TYPE="patch"
    PATCH=$((PATCH + 1))
else
    # Default: patch bump
    BUMP_TYPE="patch"
    PATCH=$((PATCH + 1))
fi

NEW_VERSION="$MAJOR.$MINOR.$PATCH"

echo ""
echo -e "${GREEN}═══════════════════════════════════════${NC}"
echo -e "${GREEN}Bump type: $BUMP_TYPE${NC}"
echo -e "${GREEN}Nueva versión: $NEW_VERSION${NC}"
echo -e "${GREEN}═══════════════════════════════════════${NC}"

# Actualizar version.txt
echo "$NEW_VERSION" > "$VERSION_FILE"

# Output para GitHub Actions
if [ -n "$GITHUB_OUTPUT" ]; then
    echo "new_version=$NEW_VERSION" >> "$GITHUB_OUTPUT"
    echo "bump_type=$BUMP_TYPE" >> "$GITHUB_OUTPUT"
fi

echo -e "${GREEN}✓ Versión actualizada a $NEW_VERSION${NC}"
