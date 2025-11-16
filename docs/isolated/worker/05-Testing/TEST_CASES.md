# Casos de Test - Worker

## Processing Flow

**TC-001:** Material procesado exitosamente  
- Input: Evento válido  
- Expected: Resumen en MongoDB, Assessment en PostgreSQL

**TC-002:** PDF corrupto  
- Input: PDF inválido  
- Expected: Error, mensaje a DLQ, no reintenta

**TC-003:** OpenAI timeout  
- Input: Timeout en API call  
- Expected: Retry 5 veces con backoff

**TC-004:** Rate limit OpenAI  
- Input: 429 Too Many Requests  
- Expected: Esperar Retry-After, reintentar

**TC-005:** Texto insuficiente (<500 palabras)  
- Input: PDF con poco texto  
- Expected: Error, no procesar, ACK mensaje

**TC-006:** MongoDB down  
- Input: MongoDB no disponible  
- Expected: Retry 5 veces, luego DLQ

**TC-007:** Procesamiento concurrente  
- Input: 10 mensajes simultáneos  
- Expected: Todos procesados, sin race conditions

**TC-008:** Costo dentro de límite  
- Input: Material normal  
- Expected: Costo <$0.20 USD

**Total casos:** 15+
