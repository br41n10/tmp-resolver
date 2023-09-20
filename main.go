package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/br41n10/tmp-resolver/store"

	"github.com/c-bata/go-prompt"
	"github.com/miekg/dns"
)

var (
	listenAddr  string
	upstreamDns string
)

func init() {
	flag.StringVar(&listenAddr, "listen", ":53", "listen on ip+port (udp)")
	flag.StringVar(&upstreamDns, "upstream", "8.8.8.8:53", "upstream dns server with port, default 8.8.8.8:53")
}

func main() {

	flag.Parse()

	db, err := store.NewLeveldbStore()
	if err != nil {
		panic(err)
	}

	dnsClient := dns.Client{Timeout: 5 * time.Second}

	dns.HandleFunc(".", func(w dns.ResponseWriter, req *dns.Msg) {

		domain := req.Question[0]

		rr, err := db.Get(Fqdn(domain.Name))
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				// 递归查询上级
				r, _, err := dnsClient.Exchange(req, upstreamDns)
				if err != nil {
					dns.HandleFailed(w, req)
					return
				}
				req.Answer = append(req.Answer, r.Answer...)
				err = w.WriteMsg(req)
				if err != nil {
					dns.HandleFailed(w, req)
					return
				}
			}
			dns.HandleFailed(w, req)
			return
		}
		req.Answer = append(req.Answer, rr)

		err = w.WriteMsg(req)
		if err != nil {
			dns.HandleFailed(w, req)
			return
		}
	})

	// start dns server
	go func() {
		err = dns.ListenAndServe(listenAddr, "udp", nil)
		if err != nil {
			panic(err)
		}
	}()

	// interactive console
	printHelp()
	for {
		input := prompt.Input(">>> ", completer)
		parts := strings.Split(input, " ")

		command := parts[0]
		switch command {
		case "set":
			if len(parts) != 4 {
				fmt.Println("input format error")
				continue
			}
			if parts[2] != "A" && parts[2] != "CNAME" {
				fmt.Println("type should be either A or CNAME")
				continue
			}
			err = setCommand(db, Fqdn(parts[1]), parts[2], parts[3])
			if err != nil {
				fmt.Printf("error set: %s\n", err.Error())
				continue
			}
		case "del":
			if len(parts) != 2 {
				fmt.Println("input format error")
				continue
			}
			err = db.Delete(Fqdn(parts[1]))
			if err != nil {
				fmt.Printf("error del: %s\n", err.Error())
				continue
			}
		case "list":
			answers, err := db.List()
			if err != nil {
				fmt.Printf("error list: %s\n", err.Error())
				continue
			}
			for _, rr := range answers {
				fmt.Println(rr.String())
			}
		case "quit", "exit":
			db.Close()
			os.Exit(0)
		}
	}
}

func completer(d prompt.Document) []prompt.Suggest {
	s := []prompt.Suggest{
		{Text: "set", Description: "set <domain> <type> <record>"},
		{Text: "del", Description: "del <domain>"},
		{Text: "list", Description: "list all"},
		{Text: "help", Description: "show help"},
	}
	return prompt.FilterHasPrefix(s, d.GetWordBeforeCursor(), true)
}

func printHelp() {
	fmt.Println("Help: ")
	fmt.Println("set <domain> <type> <record>")
	fmt.Println("del <domain>")
	fmt.Println("list")
	fmt.Println("quit/exit")
}

func setCommand(db store.Store, fqdn, ctype, record string) error {
	rr, err := dns.NewRR(fmt.Sprintf("%s %d IN %s %s", fqdn, 1, ctype, record))
	if err != nil {
		return err
	}
	return db.Set(fqdn, rr)
}

func Fqdn(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return fmt.Sprintf("%s.", domain)
}
