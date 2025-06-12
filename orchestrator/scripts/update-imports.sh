#!/bin/bash

# Update imports in lib/domain files
find src/lib/domain -name "*.ts" -type f -exec sed -i '' 's|@/domain/|@/lib/domain/|g' {} \;

# Update imports in lib/application files
find src/lib/application -name "*.ts" -type f -exec sed -i '' 's|@/domain/|@/lib/domain/|g' {} \;
find src/lib/application -name "*.ts" -type f -exec sed -i '' 's|@/application/|@/lib/application/|g' {} \;

echo "Import paths updated!"