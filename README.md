# trd01

curl -X GET http://localhost:8080/api/health \
-H "X-API-Key: secret-api-key"

curl -X POST http://localhost:8080/api/signal \
-H "Content-Type: application/json" \
-H "X-API-Key: secret-api-key" \
-d '{"data":"test signal"}'


curl -X GET http://localhost:8080/api/signal \
-H "Content-Type: application/json" \
-H "X-API-Key: secret-api-key"  
RESPONSE: {"data":"test signal","ip_address":"192.168.65.1","created_at":"2025-06-10T15:18:23.057874Z"}

curl -X POST http://localhost:8080/api/signal_file \
-H "Content-Type: multipart/form-data" \
-H "X-API-Key: secret-api-key" \
-F "file=@/Users/karel.jedlicka/Downloads/Baskets/IN/2025-06-03_basket_mpl.csv" 

