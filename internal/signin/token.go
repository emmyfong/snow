package signin

// TokenText tells the user that a saved token was not checked.
const TokenText = "Snow cannot check a token. It is saved; a wrong one fails when an agent profile's program uses it."

// Tokens returns an entry for each secret that agent profiles name and no
// provider holds, such as a gateway's token, so `snow auth login <name>`
// saves it and <NAME>_API_KEY sets it, as for a provider. A token is
// saved unchecked and gives no models.
func Tokens(names []string) []Provider {
	out := make([]Provider, len(names))
	for i, name := range names {
		out[i] = Provider{Name: name, Title: name + " token", Token: true}
	}
	return out
}
