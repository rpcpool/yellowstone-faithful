use solana_storage_proto::convert::generated;

fn make_meta_with_err(err_bytes: Vec<u8>) -> generated::TransactionStatusMeta {
    generated::TransactionStatusMeta {
        err: Some(generated::TransactionError { err: err_bytes }),
        fee: 0,
        pre_balances: vec![],
        post_balances: vec![],
        inner_instructions: vec![],
        inner_instructions_none: true,
        log_messages: vec![],
        log_messages_none: true,
        pre_token_balances: vec![],
        post_token_balances: vec![],
        rewards: vec![],
        loaded_writable_addresses: vec![],
        loaded_readonly_addresses: vec![],
        return_data: None,
        return_data_none: true,
        compute_units_consumed: None,
    }
}

#[test]
fn malformed_tx_error_returns_err() {
    let truncated = vec![8u8, 0, 0, 0];
    let meta = make_meta_with_err(truncated);

    let result: Result<solana_transaction_status::TransactionStatusMeta, _> =
        meta.try_into();

    assert!(result.is_err());
}

#[test]
fn well_formed_meta_converts_ok() {
    let meta = generated::TransactionStatusMeta {
        err: None,
        fee: 5000,
        pre_balances: vec![1, 2],
        post_balances: vec![1, 2],
        inner_instructions: vec![],
        inner_instructions_none: true,
        log_messages: vec![],
        log_messages_none: true,
        pre_token_balances: vec![],
        post_token_balances: vec![],
        rewards: vec![],
        loaded_writable_addresses: vec![],
        loaded_readonly_addresses: vec![],
        return_data: None,
        return_data_none: true,
        compute_units_consumed: Some(1234),
    };

    let result: Result<solana_transaction_status::TransactionStatusMeta, _> =
        meta.try_into();
    assert!(result.is_ok());
}
