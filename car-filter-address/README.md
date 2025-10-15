# car filter count by address

This tool scans a CAR file containing Solana blocks and filters transactions based on specified public keys. It counts the number of transactions involving these keys and provides a summary of the results.

## command-line arguments

```bash
# process a downloaded car file, output results as JSON
go run main.go \
    --car=/media/runner/cars/epoch-863.car \
    --json \
    EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB

# process a remote car file (http url), output results as JSON
go run main.go \
    --car=https://files.old-faithful.net/863/epoch-863.car \
    --json \
    EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB
```

## sample output

```json
{
  "epoch": 863,
  "keys_searched": [
    "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
    "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB"
  ],
  "num_blocks_checked": 431647,
  "num_transactions_checked": 000000,
  "results": [
    {
      "pubkey": "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
      "count_tx_success": 5951194,
      "count_tx_reverted": 7819084
    },
    {
      "pubkey": "Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB",
      "count_tx_success": 1096826,
      "count_tx_reverted": 409016
    }
  ],
  "success": true
}
```
