const web3 = require("@solana/web3.js");

test('get legacy blocks', async  () => {
    const connection = new web3.Connection('https://rpc.old-faithful.net/', 'finalized');
    const block = await connection.getBlock(209520022, {
        commitment: 'finalized',
        maxSupportedTransactionVersion: 0,
    });

    console.log('block 1 blockhash is ' + block?.blockhash);
}, 20000)
