package solanatxmetaparsers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"github.com/rpcpool/yellowstone-faithful/jsonbuilder"
	transaction_status_meta_serde_agave "github.com/rpcpool/yellowstone-faithful/parse_legacy_transaction_status_meta"
)

// https://github.com/anza-xyz/agave/blob/master/transaction-status-client-types/src/lib.rs
// https://github.com/anza-xyz/agave/blob/master/transaction-status-client-types/src/option_serializer.rs
// https://github.com/anza-xyz/agave/blob/master/transaction-status/src/lib.rs

// TransactionStatusMeta.MarshalJSON implements the json.Marshaler interface
// and is used to serialize the TransactionStatusMeta struct into JSON format.
func SerdeTransactionStatusMetaToUi(meta *transaction_status_meta_serde_agave.StoredTransactionStatusMeta) (json.RawMessage, error) {
	// Create a new JSON object
	// #[serde(rename_all = "camelCase")]
	resp := jsonbuilder.NewObject()
	{
		// .err
		// pub err: Option<TransactionError>,
		if statusErr, ok := meta.Status.(*transaction_status_meta_serde_agave.Result__Err); ok {
			status, err := ErrorToUi(
				statusErr.Value,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize error: %w", err)
			}
			resp.Apply("err", func() any {
				return status
			})
			{
				// .status
				// pub status: TransactionResult<()>, // This field is deprecated.  See https://github.com/solana-labs/solana/issues/9302
				resp.ObjectFunc("status", func(oj *jsonbuilder.OrderedJSONObject) {
					oj.Raw("Err", status)
				})
			}
		} else {
			resp.Null("err")
			resp.ObjectFunc("status", func(o *jsonbuilder.OrderedJSONObject) {
				o.Null("Ok")
			})
		}
	}

	{
		// .fee
		// pub fee: u64,
		resp.Uint("fee", meta.Fee)
	}
	{
		// .preBalances
		// pub pre_balances: Vec<u64>,
		resp.UintSlice("preBalances", meta.PreBalances)
	}
	{
		// .postBalances
		// pub post_balances: Vec<u64>,
		resp.UintSlice("postBalances", meta.PostBalances)
	}
	{
		// .innerInstructions
		// #[serde(
		//     default = "OptionSerializer::none",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub inner_instructions: OptionSerializer<Vec<UiInnerInstructions>>,
		if meta.InnerInstructions != nil && len(*meta.InnerInstructions) > 0 {
			resp.ArrayFunc(
				"innerInstructions",
				func(innerIxArray *jsonbuilder.ArrayBuilder) {
					// #[serde(rename_all = "camelCase")]
					// pub struct UiInnerInstructions {
					//     /// Transaction instruction index
					//     pub index: u8,
					//     /// List of inner instructions
					//     pub instructions: Vec<UiInstruction>,
					// }
					// 	impl From<InnerInstructions> for UiInnerInstructions {
					//     fn from(inner_instructions: InnerInstructions) -> Self {
					//         Self {
					//             index: inner_instructions.index,
					//             instructions: inner_instructions
					//                 .instructions
					//                 .iter()
					//                 .map(
					//                     |InnerInstruction {
					//                          instruction: ix,
					//                          stack_height,
					//                      }| {
					//                         UiInstruction::Compiled(UiCompiledInstruction::from(ix, *stack_height))
					//                     },
					//                 )
					//                 .collect(),
					//         }
					//     }
					// }

					// #[serde(rename_all = "camelCase", untagged)]
					// pub enum UiInstruction {
					//     Compiled(UiCompiledInstruction),
					//     Parsed(UiParsedInstruction),
					// }

					// #[serde(rename_all = "camelCase")]
					// pub struct UiCompiledInstruction {
					//     pub program_id_index: u8,
					//     pub accounts: Vec<u8>,
					//     pub data: String,
					//     pub stack_height: Option<u32>,
					// }

					// impl UiCompiledInstruction {
					//     pub fn from(instruction: &CompiledInstruction, stack_height: Option<u32>) -> Self {
					//         Self {
					//             program_id_index: instruction.program_id_index,
					//             accounts: instruction.accounts.clone(),
					//             data: bs58::encode(&instruction.data).into_string(),
					//             stack_height,
					//         }
					//     }
					// }

					if meta.InnerInstructions == nil {
						return
					}
					for _, innerInstruction := range *meta.InnerInstructions {
						if len(innerInstruction.Instructions) == 0 {
							continue
						}
						uiInnerInstruction := jsonbuilder.NewObject()
						uiInnerInstruction.Uint8("index", innerInstruction.Index)
						uiInnerInstruction.ArrayFunc(
							"instructions",
							func(arr *jsonbuilder.ArrayBuilder) {
								for _, instruction := range innerInstruction.Instructions {
									uiCompiledInstruction := jsonbuilder.NewObject()
									uiCompiledInstruction.Uint8("programIdIndex", instruction.Instruction.ProgramIdIndex)
									uiCompiledInstruction.ArrayFunc("accounts", func(accounts *jsonbuilder.ArrayBuilder) {
										for _, account := range instruction.Instruction.Accounts {
											accounts.AddUint8(account) // TODO: check if this marshals to array of numbers
										}
									})
									uiCompiledInstruction.String("data", base58.Encode(instruction.Instruction.Data[:]))
									// NOTE: stackHeight is only present in protobuf encoding.
									// if instruction.Instruction.StackHeight != nil {
									// 	uiCompiledInstruction.Uint("stackHeight", uint64(*instruction.Instruction.StackHeight))
									// }
									uiCompiledInstruction.Null("stackHeight")
									arr.AddObject(uiCompiledInstruction)
								}
							})
						innerIxArray.AddObject(uiInnerInstruction)
					}
				},
			)
		} else {
			resp.Null("innerInstructions")
		}
	}
	{
		// .logMessages
		// #[serde(
		//     default = "OptionSerializer::none",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub log_messages: OptionSerializer<Vec<String>>,
		resp.Apply("logMessages", func() any {
			if meta.LogMessages == nil {
				return nil
			}
			return *meta.LogMessages
		})
	}
	{
		// .preTokenBalances
		// #[serde(
		//     default = "OptionSerializer::none",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub pre_token_balances: OptionSerializer<Vec<UiTransactionTokenBalance>>,
		resp.ArrayFunc(
			"preTokenBalances",
			func(arr *jsonbuilder.ArrayBuilder) {
				// #[serde(rename_all = "camelCase")]
				// pub struct UiTransactionTokenBalance {
				//     pub account_index: u8,
				//     pub mint: String,
				//     pub ui_token_amount: UiTokenAmount,
				//     #[serde(
				//         default = "OptionSerializer::skip",
				//         skip_serializing_if = "OptionSerializer::should_skip"
				//     )]
				//     pub owner: OptionSerializer<String>,
				//     #[serde(
				//         default = "OptionSerializer::skip",
				//         skip_serializing_if = "OptionSerializer::should_skip"
				//     )]
				//     pub program_id: OptionSerializer<String>,
				// }
				if meta.PreTokenBalances == nil {
					return
				}
				for _, preTokenBalance := range *meta.PreTokenBalances {
					_serde_TransactionTokenBalanceToUiTransactionTokenBalance(
						preTokenBalance,
						arr,
					)
				}
			})
	}

	{
		// .postTokenBalances
		// #[serde(
		//     default = "OptionSerializer::none",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub post_token_balances: OptionSerializer<Vec<UiTransactionTokenBalance>>,
		resp.ArrayFunc(
			"postTokenBalances",
			func(arr *jsonbuilder.ArrayBuilder) {
				if meta.PostTokenBalances == nil {
					return
				}
				for _, postTokenBalance := range *meta.PostTokenBalances {
					_serde_TransactionTokenBalanceToUiTransactionTokenBalance(
						postTokenBalance,
						arr,
					)
				}
			})
	}

	{
		// .rewards
		// #[serde(
		//     default = "OptionSerializer::none",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub rewards: OptionSerializer<Rewards>,
		resp.ApplyIf(
			meta.Rewards != nil && len(*meta.Rewards) > 0,
			func(o *jsonbuilder.OrderedJSONObject) {
				o.ArrayFunc(
					"rewards",
					func(arrRewards *jsonbuilder.ArrayBuilder) {
						// pub type Rewards = Vec<Reward>;
						// #[serde(rename_all = "camelCase")]
						// pub struct Reward {
						//     pub pubkey: String,
						//     pub lamports: i64,
						//     pub post_balance: u64, // Account balance in lamports after `lamports` was applied
						//     pub reward_type: Option<RewardType>,
						//     pub commission: Option<u8>, // Vote account commission when the reward was credited, only present for voting and staking rewards
						// }

						for _, reward := range *meta.Rewards {
							uiReward := jsonbuilder.NewObject()
							{
								uiReward.String("pubkey", reward.Pubkey)
								uiReward.Int("lamports", int64(reward.Lamports))
								uiReward.Uint("postBalance", reward.PostBalance)
								if reward.RewardType != nil {
									switch (*reward.RewardType).(type) {
									case *transaction_status_meta_serde_agave.RewardType__Fee:
										uiReward.String("rewardType", "fee")
									case *transaction_status_meta_serde_agave.RewardType__Rent:
										uiReward.String("rewardType", "rent")
									case *transaction_status_meta_serde_agave.RewardType__Staking:
										uiReward.String("rewardType", "staking")
									case *transaction_status_meta_serde_agave.RewardType__Voting:
										uiReward.String("rewardType", "voting")
									default:
										panic(fmt.Errorf("unknown reward type: %T", reward.RewardType))
									}
								}
								if reward.Commission != nil {
									uiReward.Uint("commission", uint64(*reward.Commission))
								} else {
									uiReward.Null("commission")
								}
							}
							arrRewards.AddObject(uiReward)
						}
					})
			})
		if meta.Rewards == nil || len(*meta.Rewards) == 0 {
			resp.Value("rewards", make([]any, 0))
		}
	}
	{
		// .loadedAddresses
		// #[serde(
		//     default = "OptionSerializer::skip",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub loaded_addresses: OptionSerializer<UiLoadedAddresses>,
		resp.ObjectFunc(
			"loadedAddresses",
			func(o *jsonbuilder.OrderedJSONObject) {
				// #[serde(rename_all = "camelCase")]
				// pub struct UiLoadedAddresses {
				//     pub writable: Vec<String>,
				//     pub readonly: Vec<String>,
				// }
				o.ArrayFunc("writable", func(arr *jsonbuilder.ArrayBuilder) {
					for _, addr := range meta.LoadedAddresses.Writable {
						addr := solana.PublicKeyFromBytes(addr[:])
						arr.AddString(addr.String())
					}
				})
				o.ArrayFunc("readonly", func(arr *jsonbuilder.ArrayBuilder) {
					for _, addr := range meta.LoadedAddresses.Readonly {
						addr := solana.PublicKeyFromBytes(addr[:])
						arr.AddString(addr.String())
					}
				})
			})
	}
	{
		// .returnData
		// #[serde(
		//     default = "OptionSerializer::skip",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub return_data: OptionSerializer<UiTransactionReturnData>,
		resp.ApplyIf(
			meta.ReturnData != nil,
			func(o *jsonbuilder.OrderedJSONObject) {
				// #[serde(rename_all = "camelCase")]
				// pub struct UiTransactionReturnData {
				//     pub program_id: String,
				//     pub data: (String, UiReturnDataEncoding),
				// }
				o.ObjectFunc(
					"returnData",
					func(o *jsonbuilder.OrderedJSONObject) {
						pid := solana.PublicKeyFromBytes(meta.ReturnData.ProgramId[:])
						o.String("programId", pid.String())
						o.ArrayFunc("data", func(arr *jsonbuilder.ArrayBuilder) {
							arr.AddString(base64.StdEncoding.EncodeToString(meta.ReturnData.Data[:]))
							arr.AddString("base64")
						})
					})
			})
	}
	{
		// .computeUnitsConsumed
		// #[serde(
		//     default = "OptionSerializer::skip",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub compute_units_consumed: OptionSerializer<u64>,
		resp.ApplyIf(
			meta.ComputeUnitsConsumed != nil,
			func(o *jsonbuilder.OrderedJSONObject) {
				o.Uint("computeUnitsConsumed", *meta.ComputeUnitsConsumed)
			})
	}
	{
		// .costUnits
		// #[serde(
		//     default = "OptionSerializer::skip",
		//     skip_serializing_if = "OptionSerializer::should_skip"
		// )]
		// pub cost_units: OptionSerializer<u64>,
		// NOTE: this field is present only in protobuf encoding.
	}
	return resp.MarshalJSON()
}

func _serde_TransactionTokenBalanceToUiTransactionTokenBalance(
	tokenBalance transaction_status_meta_serde_agave.StoredTransactionTokenBalance,
	arr *jsonbuilder.ArrayBuilder,
) {
	// #[serde(rename_all = "camelCase")]
	// pub struct UiTransactionTokenBalance {
	//     pub account_index: u8,
	//     pub mint: String,
	//     pub ui_token_amount: UiTokenAmount,
	//     #[serde(
	//         default = "OptionSerializer::skip",
	//         skip_serializing_if = "OptionSerializer::should_skip"
	//     )]
	//     pub owner: OptionSerializer<String>,
	//     #[serde(
	//         default = "OptionSerializer::skip",
	//         skip_serializing_if = "OptionSerializer::should_skip"
	//     )]
	//     pub program_id: OptionSerializer<String>,
	// }
	// impl From<TransactionTokenBalance> for UiTransactionTokenBalance {
	//     fn from(token_balance: TransactionTokenBalance) -> Self {
	//         Self {
	//             account_index: token_balance.account_index,
	//             mint: token_balance.mint,
	//             ui_token_amount: token_balance.ui_token_amount,
	//             owner: if !token_balance.owner.is_empty() {
	//                 OptionSerializer::Some(token_balance.owner)
	//             } else {
	//                 OptionSerializer::Skip
	//             },
	//             program_id: if !token_balance.program_id.is_empty() {
	//                 OptionSerializer::Some(token_balance.program_id)
	//             } else {
	//                 OptionSerializer::Skip
	//             },
	//         }
	//     }
	// }
	// #[serde(rename_all = "camelCase")]
	// pub struct UiTokenAmount {
	//     pub ui_amount: Option<f64>,
	//     pub decimals: u8,
	//     pub amount: String,
	//     pub ui_amount_string: String,
	// }

	uiPostTokenBalance := jsonbuilder.NewObject()
	{
		uiPostTokenBalance.Uint("accountIndex", uint64(tokenBalance.AccountIndex))
		uiPostTokenBalance.String("mint", tokenBalance.Mint)
		uiPostTokenBalance.ObjectFunc("uiTokenAmount", func(o *jsonbuilder.OrderedJSONObject) {
			{
				o.Float("uiAmount", tokenBalance.UiTokenAmount.UiAmount)
			}
			o.Uint8("decimals", tokenBalance.UiTokenAmount.Decimals)
			o.String("amount", string(tokenBalance.UiTokenAmount.Amount))
			// o.String("uiAmountString", tokenBalance.UiTokenAmount.UiAmountString)// TODO: parse and format
		})
		if tokenBalance.Owner != "" {
			uiPostTokenBalance.String("owner", tokenBalance.Owner)
		}
		if tokenBalance.ProgramId != "" {
			uiPostTokenBalance.String("programId", tokenBalance.ProgramId)
		}
	}
	arr.AddObject(uiPostTokenBalance)
}
