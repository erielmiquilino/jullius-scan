package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

const receiptHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>NFC-e Teste</title>
</head>
<body>
  <div class="txtTopo">Mercado Teste LTDA</div>
  <div>CNPJ: 12.345.678/0001-90</div>
  <div>Endereco: Rua das Flores, 123</div>
  <div>Data de Emissao: 14/03/2026 12:34:56</div>
  <div>Chave de Acesso: 1234 5678 9012 3456 7890 1234 5678 9012 3456 7890 1234</div>
  <div>Valor Total: R$ 15,50</div>
  <table>
    <tr class="item">
      <td class="txtTit">Arroz Tipo 1</td>
      <td>Qtde: 1,00 UN: UN Vl. Unit: 10,00 Vl. Total: 10,00</td>
    </tr>
    <tr class="item">
      <td class="txtTit">Feijao Carioca</td>
      <td>Qtde: 1,00 UN: UN Vl. Unit: 5,50 Vl. Total: 5,50</td>
    </tr>
  </table>
</body>
</html>`

const captchaHTML = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Captcha Challenge</title>
</head>
<body>
  <h1>captcha</h1>
  <p>Verificacao de seguranca necessaria.</p>
</body>
</html>`

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/receipt.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, receiptHTML)
	})

	mux.HandleFunc("/captcha.html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, captchaHTML)
	})

	mux.HandleFunc("/slow.html", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Second)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, receiptHTML)
	})

	server := &http.Server{
		Addr:              ":8091",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Fatal(server.ListenAndServe())
}
