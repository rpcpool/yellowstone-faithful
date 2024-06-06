use {
    reader::Decoder,
    solana_sdk::{
        instruction::CompiledInstruction,
        message::{v0::LoadedAddresses, AccountKeys},
        pubkey::Pubkey,
    },
    solana_transaction_status::parse_instruction::parse,
};

pub mod byte_order;
pub mod reader;
pub mod type_size;

/// # Safety
/// Bytes length should have at least len size.
#[no_mangle]
pub unsafe extern "C" fn parse_instruction(bytes: *const u8, len: usize) -> Response {
    // let started_at = Instant::now();
    let bytes = {
        assert!(!bytes.is_null());
        std::slice::from_raw_parts(bytes, len)
    };
    let bytes = bytes.to_vec();
    // println!("[rust] params raw bytes: {:?}", bytes);
    // println!("[rust] params:");
    let mut decoder = Decoder::new(bytes);
    {
        // read program ID:
        let program_id_bytes = decoder.read_bytes(32).unwrap();
        let program_id =
            solana_sdk::pubkey::Pubkey::try_from(program_id_bytes).expect("invalid program id");
        let mut instruction = CompiledInstruction {
            program_id_index: 0,
            accounts: vec![],
            data: vec![],
        };
        {
            instruction.program_id_index = decoder.read_u8().unwrap();
            let accounts_len = decoder.read_u16().unwrap() as usize;
            for _ in 0..accounts_len {
                let account_index = decoder.read_u8().unwrap();
                instruction.accounts.push(account_index);
            }
            let data_len = decoder.read_u16().unwrap() as usize;
            for _ in 0..data_len {
                let data_byte = decoder.read_u8().unwrap();
                instruction.data.push(data_byte);
            }
        }

        let static_account_keys_len = decoder.read_u16().unwrap() as usize;
        // println!(
        //     "[rust] static_account_keys_len: {:?}",
        //     static_account_keys_len
        // );
        let mut static_account_keys_vec = vec![];
        for _ in 0..static_account_keys_len {
            let account_key_bytes = decoder.read_bytes(32).unwrap();
            let account_key = solana_sdk::pubkey::Pubkey::try_from(account_key_bytes)
                .expect("invalid account key in static account keys");
            static_account_keys_vec.push(account_key);
        }

        let has_dynamic_account_keys = decoder.read_option().unwrap();
        let parsed_account_keys: Combined = if has_dynamic_account_keys {
            let mut loaded_addresses = LoadedAddresses::default();
            let num_writable_accounts = decoder.read_u16().unwrap() as usize;
            // println!("[rust] num_writable_accounts: {:?}", num_writable_accounts);
            // read 32 bytes for each writable account:
            for dyn_wri_index in 0..num_writable_accounts {
                let account_key_bytes = decoder.read_bytes(32);
                if account_key_bytes.is_err() {
                    // println!("[rust] account_key_bytes error: {:?}", account_key_bytes);
                    let mut response = vec![0; 32];
                    // add error string to response:
                    let error = account_key_bytes.err().unwrap();
                    let error = format!(
                        "account_key_bytes error at index: {:?}: {:?}",
                        dyn_wri_index, error
                    );
                    response.extend_from_slice(error.as_bytes());
                    let data = response.as_mut_ptr();
                    let len = response.len();

                    return Response {
                        buf: Buffer {
                            data: unsafe { data.add(32) },
                            len: len - 32,
                        },
                        status: 1,
                    };
                }
                let account_key_bytes = account_key_bytes.unwrap();
                let account_key = solana_sdk::pubkey::Pubkey::try_from(account_key_bytes)
                    .expect("invalid account key in writable accounts");
                loaded_addresses.writable.push(account_key);
            }
            let num_readonly_accounts = decoder.read_u16().unwrap() as usize;
            // read 32 bytes for each readonly account:
            for _ in 0..num_readonly_accounts {
                let account_key_bytes = decoder.read_bytes(32).unwrap();
                let account_key = solana_sdk::pubkey::Pubkey::try_from(account_key_bytes)
                    .expect("invalid account key in readonly accounts");
                loaded_addresses.readonly.push(account_key);
            }

            Combined {
                parent: static_account_keys_vec,
                child: Some(loaded_addresses),
            }
        } else {
            Combined {
                parent: static_account_keys_vec,
                child: None,
            }
        };
        let sommmm = &parsed_account_keys.child.unwrap_or_default();

        let account_keys = AccountKeys::new(
            &parsed_account_keys.parent,
            if has_dynamic_account_keys {
                Some(sommmm)
            } else {
                None
            },
        );

        let mut stack_height: Option<u32> = None;
        {
            let has_stack_height = decoder.read_option().unwrap();
            // println!("[rust] has_stack_height: {:?}", has_stack_height);
            if has_stack_height {
                stack_height = Some(
                    decoder
                        .read_u32(byte_order::ByteOrder::LittleEndian)
                        .unwrap(),
                );
                // println!("[rust] stack_height: {:?}", stack_height);
            }
        }
        // println!("[rust] program_id: {:?}", program_id);
        // println!("[rust] instruction: {:?}", instruction);
        // println!(
        //     "[rust] account_keys.static: {:?}",
        //     parsed_account_keys.parent
        // );
        // println!(
        //     "[rust] has_dynamic_account_keys: {:?}",
        //     has_dynamic_account_keys
        // );
        // println!("[rust] account_keys.dynamic: {:?}", sommmm);
        // println!("[rust] stack_height: {:?}", stack_height);

        let parsed = parse(
            &program_id, // program_id
            &instruction,
            &account_keys,
            stack_height,
        );
        if parsed.is_err() {
            // println!("[rust] parse error: {:?}", parsed);
            let mut response = vec![0; 32];
            // add error string to response:
            let error = parsed.err().unwrap();
            let error = format!("{:?}", error);
            response.extend_from_slice(error.as_bytes());
            let data = response.as_mut_ptr();
            let len = response.len();

            Response {
                buf: Buffer {
                    data: unsafe { data.add(32) },
                    len: len - 32,
                },
                status: 1,
            }
        } else {
            // println!(
            //     "[rust] successfully parsed the instruction in {:?}: {:?}",
            //     Instant::now() - started_at,
            //     parsed
            // );
            let parsed = parsed.unwrap();
            let parsed_json = serde_json::to_vec(&parsed).unwrap();
            {
                // let parsed_json_str = String::from_utf8(parsed_json.clone()).unwrap();
                // println!(
                //     "[rust] parsed instruction as json at {:?}: {}",
                //     Instant::now() - started_at,
                //     parsed_json_str
                // );
            }

            // println!("[rust] {:?}", Instant::now() - started_at);
            let mut response = vec![0; 32];
            response.extend_from_slice(&parsed_json);

            let data = response.as_mut_ptr();
            let len = response.len();
            // println!("[rust] {:?}", Instant::now() - started_at);
            Response {
                buf: Buffer {
                    data: unsafe { data.add(32) },
                    len: len - 32,
                },
                status: 0,
            }
        }
    }
}

#[repr(C)]
pub struct Response {
    buf: Buffer,
    status: i32,
}

#[repr(C)]
struct Buffer {
    data: *mut u8,
    len: usize,
}

#[derive(Default)]
struct Combined {
    parent: Vec<Pubkey>,
    child: Option<LoadedAddresses>,
}
