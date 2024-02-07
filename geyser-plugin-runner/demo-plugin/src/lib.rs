use solana_geyser_plugin_interface::geyser_plugin_interface::{
    GeyserPlugin, ReplicaAccountInfoVersions, ReplicaBlockInfoVersions, ReplicaEntryInfoVersions,
    ReplicaTransactionInfoVersions, Result, SlotStatus,
};

#[no_mangle]
#[allow(improper_ctypes_definitions)]
/// # Safety
///
/// This function returns the GeyserPluginPostgres pointer as trait GeyserPlugin.
pub unsafe extern "C" fn _create_plugin() -> *mut dyn GeyserPlugin {
    let plugin = GeyserPluginDemo::new();
    let plugin: Box<dyn GeyserPlugin> = Box::new(plugin);
    Box::into_raw(plugin)
}

#[derive(Default)]
pub struct GeyserPluginDemo {}

impl GeyserPluginDemo {
    pub fn new() -> Self {
        Self::default()
    }
}

impl std::fmt::Debug for GeyserPluginDemo {
    fn fmt(&self, _: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        Ok(())
    }
}

fn green_bg(s: &str) -> String {
    // green BG, black FG
    format!("\x1b[42;30m{}\x1b[0m", s)
}

fn blue_bg(s: &str) -> String {
    // blue BG, black FG
    format!("\x1b[44;30m{}\x1b[0m", s)
}

fn cyan_bg(s: &str) -> String {
    // cyan BG, black FG
    format!("\x1b[46;30m{}\x1b[0m", s)
}

fn white_bg(s: &str) -> String {
    // white BG, black FG
    format!("\x1b[47;30m{}\x1b[0m", s)
}

fn purple_bg(s: &str) -> String {
    // purple BG, black FG
    format!("\x1b[45;30m{}\x1b[0m", s)
}

const BANNER: &str = "::plugin::";

impl GeyserPlugin for GeyserPluginDemo {
    fn name(&self) -> &'static str {
        "plugin::GeyserPluginDemo"
    }

    fn on_load(&mut self, config_file: &str) -> Result<()> {
        println!(
            "{} Loading plugin: {:?} from config_file {:?}",
            green_bg(BANNER),
            self.name(),
            config_file
        );
        Ok(())
    }

    fn on_unload(&mut self) {
        println!("{} Unloading plugin: {:?}", green_bg(BANNER), self.name());
    }

    fn notify_end_of_startup(&self) -> Result<()> {
        println!(
            "{} Notifying the end of startup for accounts notifications",
            green_bg(BANNER),
        );
        Ok(())
    }

    /// Check if the plugin is interested in account data
    /// Default is true -- if the plugin is not interested in
    /// account data, please return false.
    fn account_data_notifications_enabled(&self) -> bool {
        false
    }

    /// Check if the plugin is interested in transaction data
    fn transaction_notifications_enabled(&self) -> bool {
        true
    }

    fn entry_notifications_enabled(&self) -> bool {
        true
    }

    fn update_account(
        &self,
        _account: ReplicaAccountInfoVersions,
        slot: u64,
        _is_startup: bool,
    ) -> Result<()> {
        // NOTE: account updates are NOT supported in old-faithful.
        println!("{} Updating account at slot {:?}", green_bg(BANNER), slot);
        Ok(())
    }

    fn notify_entry(&self, _entry: ReplicaEntryInfoVersions) -> Result<()> {
        println!(
            "{} Received ENTRY: {:?}",
            blue_bg(BANNER),
            match _entry {
                ReplicaEntryInfoVersions::V0_0_1(entry_info) => entry_info,
            },
        );
        Ok(())
    }

    fn notify_transaction(
        &self,
        _transaction_info: ReplicaTransactionInfoVersions,
        slot: u64,
    ) -> Result<()> {
        println!(
            "{} Received TRANSACTION at slot {:?}, signature {:?}",
            cyan_bg(BANNER),
            slot,
            match _transaction_info {
                ReplicaTransactionInfoVersions::V0_0_1(transaction_info) => {
                    transaction_info.signature
                }
                ReplicaTransactionInfoVersions::V0_0_2(transaction_info) => {
                    transaction_info.signature
                }
            }
        );
        Ok(())
    }

    fn update_slot_status(
        &self,
        slot: u64,
        _parent: Option<u64>,
        status: SlotStatus,
    ) -> Result<()> {
        println!(
            "{} Updating slot {:?} with status {:?}",
            purple_bg(BANNER),
            slot,
            status
        );
        Ok(())
    }

    fn notify_block_metadata(&self, _block_info: ReplicaBlockInfoVersions) -> Result<()> {
        println!(
            "{} Notifying block metadata: slot {}, blocktime {:?}",
            white_bg(BANNER),
            match _block_info {
                ReplicaBlockInfoVersions::V0_0_1(block_info) => block_info.slot,
                ReplicaBlockInfoVersions::V0_0_2(block_info) => block_info.slot,
                ReplicaBlockInfoVersions::V0_0_3(block_info) => block_info.slot,
            },
            match _block_info {
                ReplicaBlockInfoVersions::V0_0_1(block_info) => block_info.block_time,
                ReplicaBlockInfoVersions::V0_0_2(block_info) => block_info.block_time,
                ReplicaBlockInfoVersions::V0_0_3(block_info) => block_info.block_time,
            },
        );
        Ok(())
    }
}
