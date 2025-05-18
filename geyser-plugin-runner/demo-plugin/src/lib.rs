use {
    agave_geyser_plugin_interface::geyser_plugin_interface::{
        GeyserPlugin, ReplicaAccountInfoVersions, ReplicaBlockInfoVersions,
        ReplicaEntryInfoVersions, ReplicaTransactionInfoVersions, Result, SlotStatus,
    },
    colored::Colorize,
    tracing::info,
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

const BANNER: &str = "::plugin::";

use tracing_subscriber::{fmt, EnvFilter};

impl GeyserPlugin for GeyserPluginDemo {
    fn name(&self) -> &'static str {
        "plugin::GeyserPluginDemo"
    }

    fn on_load(&mut self, config_file: &str, _is_reload: bool) -> Result<()> {
        fmt().with_env_filter(EnvFilter::from_default_env()).init();
        info!(
            "{} Loading plugin: {:?} from config_file {:?}",
            BANNER.green().on_black(),
            self.name(),
            config_file
        );
        Ok(())
    }

    fn on_unload(&mut self) {
        info!(
            "{} Unloading plugin: {:?}",
            BANNER.green().on_black(),
            self.name()
        );
    }

    fn notify_end_of_startup(&self) -> Result<()> {
        info!(
            "{} Notifying the end of startup for accounts notifications",
            BANNER.green().on_black(),
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
        info!(
            "{} Updating account at slot {:?}",
            BANNER.green().on_black(),
            slot
        );
        Ok(())
    }

    fn notify_entry(&self, _entry: ReplicaEntryInfoVersions) -> Result<()> {
        match _entry {
            ReplicaEntryInfoVersions::V0_0_1(entry_info) => {
                info!(
                    "{} Received ENTRY: {:?}",
                    BANNER.blue().on_black(),
                    entry_info,
                );
            }
            ReplicaEntryInfoVersions::V0_0_2(entry_info) => {
                info!(
                    "{} Received ENTRY: {:?}",
                    BANNER.blue().on_black(),
                    entry_info,
                );
            }
        };
        Ok(())
    }

    fn notify_transaction(
        &self,
        _transaction_info: ReplicaTransactionInfoVersions,
        slot: u64,
    ) -> Result<()> {
        info!(
            "{} Received TRANSACTION at slot {:?}, signature {:?}",
            BANNER.cyan().on_black(),
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
        status: &SlotStatus,
    ) -> Result<()> {
        info!(
            "{} Updating slot {:?} with status {:?}",
            BANNER.purple().on_black(),
            slot,
            status
        );
        Ok(())
    }

    fn notify_block_metadata(&self, _block_info: ReplicaBlockInfoVersions) -> Result<()> {
        info!(
            "{} Notifying block metadata: slot {}, blocktime {:?}",
            BANNER.white().on_black(),
            match _block_info {
                ReplicaBlockInfoVersions::V0_0_1(block_info) => block_info.slot,
                ReplicaBlockInfoVersions::V0_0_2(block_info) => block_info.slot,
                ReplicaBlockInfoVersions::V0_0_3(block_info) => block_info.slot,
                ReplicaBlockInfoVersions::V0_0_4(block_info) => block_info.slot,
            },
            match _block_info {
                ReplicaBlockInfoVersions::V0_0_1(block_info) => block_info.block_time,
                ReplicaBlockInfoVersions::V0_0_2(block_info) => block_info.block_time,
                ReplicaBlockInfoVersions::V0_0_3(block_info) => block_info.block_time,
                ReplicaBlockInfoVersions::V0_0_4(block_info) => block_info.block_time,
            },
        );
        Ok(())
    }
}
