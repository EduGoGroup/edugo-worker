# Validación Sprint 06
```bash
# Pipeline verde
gh run list --workflow=ci.yml

# Docker build
docker build -t edugo-worker:test .
docker run edugo-worker:test
```
## Criterios
- [ ] Pipeline ejecuta en cada push
- [ ] Tests automáticos
- [ ] Docker build exitoso
