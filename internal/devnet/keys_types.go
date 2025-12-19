package devnet

import "fmt"

// ValidatorKey represents a validator's key information.
type ValidatorKey struct {
	Index int `json:"index"`

	// Account key (secp256k1/ethsecp256k1)
	Address           string `json:"address"`                       // stable1... (account address)
	AddressHex        string `json:"address_hex"`                   // 0x... (EVM address)
	ValoperAddr       string `json:"valoper_address"`               // stablevaloper1...
	AccountPubKey     string `json:"account_pubkey,omitempty"`      // base64 encoded account pubkey
	AccountPrivKey    string `json:"account_privkey,omitempty"`     // hex encoded account private key
	AccountPrivKeyHex string `json:"account_privkey_hex,omitempty"` // hex private key for EVM (without 0x prefix)

	// Consensus key (ed25519)
	ValconsAddr    string `json:"valcons_address"`            // stablevalcons1...
	ConsPubKey     string `json:"cons_pubkey"`                // base64 encoded consensus pubkey
	ConsAddrHex    string `json:"cons_address_hex"`           // hex consensus address
	ConsPrivKeyHex string `json:"cons_privkey_hex,omitempty"` // hex consensus private key

	Mnemonic string `json:"mnemonic,omitempty"`
}

// AccountKey represents an account's key information.
type AccountKey struct {
	Index      int    `json:"index"`
	Address    string `json:"address"`           // stable1...
	AddressHex string `json:"address_hex"`       // 0x... (EVM address)
	PubKey     string `json:"pubkey,omitempty"`  // base64 encoded pubkey
	PrivKey    string `json:"privkey,omitempty"` // base64 encoded private key (if available)
	Mnemonic   string `json:"mnemonic,omitempty"`
}

// KeyExport contains all exported keys.
type KeyExport struct {
	Validators []ValidatorKey `json:"validators"`
	Accounts   []AccountKey   `json:"accounts"`
}

// FormatKeysText formats keys for text output.
func FormatKeysText(export *KeyExport) string {
	var output string

	if len(export.Validators) > 0 {
		output += "Validator Keys\n"
		output += "══════════════════════════════════════════════════════════════════════════════\n"
		for i := range export.Validators {
			v := &export.Validators[i]
			output += fmt.Sprintf("Node %d:\n", v.Index)
			output += fmt.Sprintf("  Account Address:     %s\n", v.Address)
			output += fmt.Sprintf("  Account Address HEX: %s\n", v.AddressHex)
			output += fmt.Sprintf("  Valoper Address:     %s\n", v.ValoperAddr)
			if v.AccountPubKey != "" {
				output += fmt.Sprintf("  Account PubKey:      %s\n", v.AccountPubKey)
			}
			output += "  ─────────────────────────────────────────────────────────────────────────\n"
			if v.ValconsAddr != "" {
				output += fmt.Sprintf("  Valcons Address:     %s\n", v.ValconsAddr)
			}
			if v.ConsAddrHex != "" {
				output += fmt.Sprintf("  Cons Address HEX:    %s\n", v.ConsAddrHex)
			}
			if v.ConsPubKey != "" {
				output += fmt.Sprintf("  Consensus PubKey:    %s\n", v.ConsPubKey)
			}
			if v.ConsPrivKeyHex != "" {
				output += fmt.Sprintf("  Cons PrivKey:        %s\n", v.ConsPrivKeyHex)
			}
			if v.Mnemonic != "" {
				output += "  ─────────────────────────────────────────────────────────────────────────\n"
				output += fmt.Sprintf("  Mnemonic:            %s\n", v.Mnemonic)
			}
			output += "\n"
		}
	}

	if len(export.Accounts) > 0 {
		output += "Account Keys\n"
		output += "══════════════════════════════════════════════════════════════════════════════\n"
		for _, a := range export.Accounts {
			output += fmt.Sprintf("Account %d:\n", a.Index)
			output += fmt.Sprintf("  Address:             %s\n", a.Address)
			output += fmt.Sprintf("  Address HEX:         %s\n", a.AddressHex)
			if a.PubKey != "" {
				output += fmt.Sprintf("  PubKey:              %s\n", a.PubKey)
			}
			if a.Mnemonic != "" {
				output += fmt.Sprintf("  Mnemonic:            %s\n", a.Mnemonic)
			}
			output += "\n"
		}
	}

	return output
}

// FormatKeysEnv formats keys for environment variable export.
func FormatKeysEnv(export *KeyExport) string {
	var output string

	for i := range export.Validators {
		v := &export.Validators[i]
		output += fmt.Sprintf("export VALIDATOR_%d_ADDRESS=\"%s\"\n", v.Index, v.Address)
		output += fmt.Sprintf("export VALIDATOR_%d_ADDRESS_HEX=\"%s\"\n", v.Index, v.AddressHex)
		output += fmt.Sprintf("export VALIDATOR_%d_VALOPER=\"%s\"\n", v.Index, v.ValoperAddr)
		if v.AccountPubKey != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_ACCOUNT_PUBKEY=\"%s\"\n", v.Index, v.AccountPubKey)
		}
		if v.ValconsAddr != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_VALCONS=\"%s\"\n", v.Index, v.ValconsAddr)
		}
		if v.ConsAddrHex != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_CONS_ADDRESS_HEX=\"%s\"\n", v.Index, v.ConsAddrHex)
		}
		if v.ConsPubKey != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_CONS_PUBKEY=\"%s\"\n", v.Index, v.ConsPubKey)
		}
		if v.ConsPrivKeyHex != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_CONS_PRIVKEY=\"%s\"\n", v.Index, v.ConsPrivKeyHex)
		}
		if v.Mnemonic != "" {
			output += fmt.Sprintf("export VALIDATOR_%d_MNEMONIC=\"%s\"\n", v.Index, v.Mnemonic)
		}
	}

	for _, a := range export.Accounts {
		output += fmt.Sprintf("export ACCOUNT_%d_ADDRESS=\"%s\"\n", a.Index, a.Address)
		output += fmt.Sprintf("export ACCOUNT_%d_ADDRESS_HEX=\"%s\"\n", a.Index, a.AddressHex)
		if a.PubKey != "" {
			output += fmt.Sprintf("export ACCOUNT_%d_PUBKEY=\"%s\"\n", a.Index, a.PubKey)
		}
		if a.Mnemonic != "" {
			output += fmt.Sprintf("export ACCOUNT_%d_MNEMONIC=\"%s\"\n", a.Index, a.Mnemonic)
		}
	}

	return output
}
