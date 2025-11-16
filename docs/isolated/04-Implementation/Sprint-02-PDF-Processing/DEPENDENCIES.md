# Dependencias Sprint 02

## Dependencias Técnicas
- [ ] pdftotext instalado (poppler-utils)
- [ ] tesseract instalado (OCR)
- [ ] AWS SDK para S3

```bash
# Instalar pdftotext
brew install poppler  # macOS
apt-get install poppler-utils  # Linux

# Instalar tesseract
brew install tesseract
apt-get install tesseract-ocr tesseract-ocr-spa

# Verificar
pdftotext -v
tesseract --version

# AWS SDK Go
go get github.com/aws/aws-sdk-go-v2/service/s3@latest
```

## Variables de Entorno
```bash
export AWS_REGION="us-east-1"
export AWS_ACCESS_KEY_ID="..."
export AWS_SECRET_ACCESS_KEY="..."
export S3_BUCKET="edugo-materials"
```
