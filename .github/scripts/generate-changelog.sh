#!/bin/bash
# Script para generar CHANGELOG desde commits

set -e

CHANGELOG_FILE="CHANGELOG.md"
NEW_VERSION="$1"

if [ -z "$NEW_VERSION" ]; then
    echo "Error: Debes especificar la nueva versión"
    echo "Uso: $0 <version>"
    exit 1
fi

# Obtener último tag
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

if [ -z "$LAST_TAG" ]; then
    COMMIT_RANGE="HEAD"
else
    COMMIT_RANGE="$LAST_TAG..HEAD"
fi

# Fecha actual
DATE=$(date +%Y-%m-%d)

# Crear sección temporal
TEMP_FILE=$(mktemp)

echo "## [$NEW_VERSION] - $DATE" > "$TEMP_FILE"
echo "" >> "$TEMP_FILE"

# Recopilar commits por categoría
FEATURES=$(git log $COMMIT_RANGE --pretty=format:"%s" | grep "^feat:" | sed 's/^feat: /- /' || true)
FIXES=$(git log $COMMIT_RANGE --pretty=format:"%s" | grep "^fix:" | sed 's/^fix: /- /' || true)
BREAKING=$(git log $COMMIT_RANGE --pretty=format:"%s" --grep="BREAKING CHANGE" || true)
CHORES=$(git log $COMMIT_RANGE --pretty=format:"%s" | grep "^chore:" | sed 's/^chore: /- /' || true)
DOCS=$(git log $COMMIT_RANGE --pretty=format:"%s" | grep "^docs:" | sed 's/^docs: /- /' || true)

# Agregar secciones solo si tienen contenido
if [ -n "$BREAKING" ]; then
    echo "### ⚠️ BREAKING CHANGES" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "$BREAKING" | sed 's/^/- /' >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
fi

if [ -n "$FEATURES" ]; then
    echo "### Added" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "$FEATURES" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
fi

if [ -n "$FIXES" ]; then
    echo "### Fixed" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "$FIXES" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
fi

if [ -n "$CHORES" ]; then
    echo "### Changed" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "$CHORES" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
fi

if [ -n "$DOCS" ]; then
    echo "### Documentation" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
    echo "$DOCS" >> "$TEMP_FILE"
    echo "" >> "$TEMP_FILE"
fi

# Insertar en CHANGELOG.md después de [Unreleased]
if [ -f "$CHANGELOG_FILE" ]; then
    # Crear backup
    cp "$CHANGELOG_FILE" "${CHANGELOG_FILE}.bak"

    # Insertar nueva sección después de ## [Unreleased]
    awk -v new_section="$(cat $TEMP_FILE)" '
        /^## \[Unreleased\]/ {
            print
            print ""
            print new_section
            next
        }
        {print}
    ' "${CHANGELOG_FILE}.bak" > "$CHANGELOG_FILE"

    rm "${CHANGELOG_FILE}.bak"
else
    # Crear CHANGELOG nuevo
    cat > "$CHANGELOG_FILE" << EOF
# Changelog

## [Unreleased]

$(cat $TEMP_FILE)

[$NEW_VERSION]: https://github.com/EduGoGroup/edugo-worker/releases/tag/v$NEW_VERSION
EOF
fi

rm "$TEMP_FILE"

echo "✓ CHANGELOG actualizado con versión $NEW_VERSION"
