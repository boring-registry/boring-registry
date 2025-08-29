#!/bin/bash

set -e

echo "Auto Release Script - Internal Versioning"

if [ ! -f "INTERNAL_VERSION" ]; then
    echo "0.0.0" > INTERNAL_VERSION
fi

CURRENT_VERSION=$(cat INTERNAL_VERSION)
echo "Current internal version: $CURRENT_VERSION"

if [[ $CURRENT_VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    MAJOR=${BASH_REMATCH[1]}
    MINOR=${BASH_REMATCH[2]}
    PATCH=${BASH_REMATCH[3]}
else
    echo "Invalid version format in INTERNAL_VERSION: $CURRENT_VERSION"
    echo "Expected format: 1.2.3"
    exit 1
fi

LATEST_INTERNAL_TAG="confluent-v$CURRENT_VERSION"
if git rev-parse "$LATEST_INTERNAL_TAG" >/dev/null 2>&1; then
    COMMITS_SINCE_TAG=$(git log $LATEST_INTERNAL_TAG..HEAD --oneline)
else
    echo "First release - analyzing all recent commits"
    COMMITS_SINCE_TAG=$(git log --oneline -5)
fi
COMMIT_COUNT=$(echo "$COMMITS_SINCE_TAG" | wc -l | tr -d ' ')

if [ -z "$COMMITS_SINCE_TAG" ] || [ "$COMMIT_COUNT" -eq 0 ]; then
    echo "No new commits since $LATEST_INTERNAL_TAG"
    exit 0
fi

echo "Commits since last internal release:"
echo "$COMMITS_SINCE_TAG"
echo ""

BUMP_TYPE="patch"

if echo "$COMMITS_SINCE_TAG" | grep -i -E "BREAKING|breaking change|major:" >/dev/null; then
    BUMP_TYPE="major"
elif echo "$COMMITS_SINCE_TAG" | grep -i -E "feat:|feature:|new:" >/dev/null; then
    BUMP_TYPE="minor"
elif echo "$COMMITS_SINCE_TAG" | grep -i -E "fix:|bug:|patch:" >/dev/null; then
    BUMP_TYPE="patch"
else
    echo "No recognizable change type found. Defaulting to patch release."
fi

NEW_MAJOR=$MAJOR
NEW_MINOR=$MINOR  
NEW_PATCH=$PATCH

case $BUMP_TYPE in
    "major")
        NEW_MAJOR=$((MAJOR + 1))
        NEW_MINOR=0
        NEW_PATCH=0
        ;;
    "minor")
        NEW_MINOR=$((MINOR + 1))
        NEW_PATCH=0
        ;;
    "patch")
        NEW_PATCH=$((PATCH + 1))
        ;;
esac

NEW_VERSION="$NEW_MAJOR.$NEW_MINOR.$NEW_PATCH"
NEW_TAG="confluent-v$NEW_VERSION"

echo "Version bump: $CURRENT_VERSION → $NEW_VERSION ($BUMP_TYPE)"



echo "Updating INTERNAL_VERSION file..."
echo "$NEW_VERSION" > INTERNAL_VERSION

echo "Creating internal tag $NEW_TAG..."
git add INTERNAL_VERSION
git commit -m "chore: bump version to $NEW_VERSION [skip ci]"
git tag -a "$NEW_TAG" -m "Internal Release $NEW_VERSION

Commits included:
$COMMITS_SINCE_TAG

Type: $BUMP_TYPE"

echo "Pushing version update and tag..."
git push origin main
git push origin "$NEW_TAG"

echo "Successfully created and pushed internal version $NEW_VERSION"

echo "Version bump and git operations complete!"
echo "Docker images will be built by separate architecture-specific jobs"
echo "Note: This uses separate versioning from upstream"
echo "Upstream workflows are not triggered by confluent-v* tags"
