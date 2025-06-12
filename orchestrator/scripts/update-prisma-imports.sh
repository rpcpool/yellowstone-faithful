#!/bin/bash

# Update imports to use new prisma location
find src -name "*.ts" -o -name "*.tsx" | xargs sed -i '' 's|from '\''@/lib/prisma'\''|from '\''@/lib/infrastructure/persistence/prisma'\''|g'
find src -name "*.ts" -o -name "*.tsx" | xargs sed -i '' 's|from "@/lib/prisma"|from "@/lib/infrastructure/persistence/prisma"|g'

echo "Prisma imports updated!"