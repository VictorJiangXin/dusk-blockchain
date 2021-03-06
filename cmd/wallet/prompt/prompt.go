package prompt

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/dusk-network/dusk-blockchain/cmd/wallet/conf"
	"github.com/dusk-network/dusk-blockchain/pkg/core/data/wallet"
	"github.com/dusk-network/dusk-protobuf/autogen/go/node"
	"github.com/manifoldco/promptui"
)

// LoadMenu opens the prompt for loading a wallet.
func LoadMenu(client node.WalletClient) error {

	prompt := promptui.Select{
		Label: "Select action",
		Items: []string{"Load Wallet", "Create Wallet", "Load Wallet From Seed", "Exit"},
	}

	_, result, err := prompt.Run()
	if err != nil {
		// Prompts should always succeed.
		panic(err)
	}

	var resp *node.LoadResponse
	switch result {
	case "Load Wallet":
		resp, err = loadWallet(client)
	case "Create Wallet":
		resp, err = createWallet(client)
	case "Load Wallet From Seed":
		resp, err = loadFromSeed(client)
	case "Exit":
		os.Exit(0)
	}

	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(os.Stdout, string(resp.Key.PublicKey))
	return nil
}

// WalletMenu opens the prompt for doing wallet operations.
func WalletMenu(client *conf.NodeClient) error {
	for {
		// Get sync progress first and print it
		resp, err := client.ChainClient.GetSyncProgress(context.Background(), &node.EmptyRequest{})
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(os.Stdout, "sync progress: %.2f ", resp.Progress)

		prompt := promptui.Select{
			Label: "Select action",
			Items: []string{"Transfer DUSK", "Stake DUSK", "Bid DUSK", "Show Balance", "Show Address", "Show Transaction History", "Automate Provisioner Participation", "Automate Block Generator Participation", "Exit"},
			Size:  8,
		}

		_, result, err := prompt.Run()
		if err != nil {
			panic(err)
		}

		var res string
		switch result {
		case "Transfer DUSK":
			resp, err := transferDusk(client.TransactorClient)
			if err != nil {
				return err
			}

			res = "Tx hash: " + hex.EncodeToString(resp.Hash)
		case "Stake DUSK":
			resp, err := stakeDusk(client.TransactorClient)
			if err != nil {
				return err
			}

			res = "Tx hash: " + hex.EncodeToString(resp.Hash)
		case "Bid DUSK":
			resp, err := bidDusk(client.TransactorClient)
			if err != nil {
				return err
			}

			res = "Tx hash: " + hex.EncodeToString(resp.Hash)
		case "Show Balance":
			resp, err := client.WalletClient.GetBalance(context.Background(), &node.EmptyRequest{})
			if err != nil {
				return err
			}

			res = fmt.Sprintf("Unlocked balance: %.8f\nLocked balance: %.8f\n", float64(resp.UnlockedBalance)/float64(wallet.DUSK), float64(resp.LockedBalance)/float64(wallet.DUSK))
		case "Show Address":
			resp, err := client.WalletClient.GetAddress(context.Background(), &node.EmptyRequest{})
			if err != nil {
				return err
			}

			res = "Address: " + string(resp.Key.PublicKey)
		case "Show Transaction History":
			resp, err := client.WalletClient.GetTxHistory(context.Background(), &node.EmptyRequest{})
			if err != nil {
				return err
			}

			s := formatRecords(resp)
			res = s.String()
		case "Automate Provisioner Participation":
			resp, err := client.ProvisionerClient.AutomateStakes(context.Background(), &node.EmptyRequest{})
			if err != nil {
				return err
			}

			res = resp.Response
		case "Automate Block Generator Participation":
			resp, err := client.BlockGeneratorClient.AutomateBids(context.Background(), &node.EmptyRequest{})
			if err != nil {
				return err
			}

			res = resp.Response
		case "Exit":
			os.Exit(0)
		}

		_, _ = fmt.Fprintln(os.Stdout, res)
	}
}

func formatRecords(resp *node.TxHistoryResponse) strings.Builder {
	s := strings.Builder{}
	for _, record := range resp.Records {
		// Type
		_, _ = s.WriteString(node.TxType_name[int32(record.Type)])
		// Height
		_, _ = s.WriteString(strconv.FormatUint(record.Height, 10) + " / ")
		if record.Direction == node.Direction_IN {
			_, _ = s.WriteString("IN / ")
		} else {
			_, _ = s.WriteString("OUT / ")
		}
		// Time
		_, _ = s.WriteString(time.Unix(record.Timestamp, 0).Format(time.UnixDate) + " / ")
		// Amount
		_, _ = s.WriteString(fmt.Sprintf("%.8f DUSK", float64(record.Amount)/float64(wallet.DUSK)) + " / ")
		// Fee
		_, _ = s.WriteString(fmt.Sprintf("%.8f DUSK", float64(record.Fee)/float64(wallet.DUSK)) + " / ")
		// Unlock height
		_, _ = s.WriteString("Unlocks at " + strconv.FormatUint(record.UnlockHeight, 10) + " / ")
		// Hash
		_, _ = s.WriteString("Hash: " + hex.EncodeToString(record.Hash) + " / ")
		// Data
		_, _ = s.WriteString("Call data: " + hex.EncodeToString(record.Data) + " / ")
		// Obfuscated
		if record.Obfuscated {
			_, _ = s.WriteString("Obfuscated")
		} else {
			_, _ = s.WriteString("Transparent")
		}

		_, _ = s.WriteString("\n")
	}

	return s
}
