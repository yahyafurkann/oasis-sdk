use std::sync::Arc;

use oasis_core_runtime::{
    common::crypto::hash,
    storage,
    storage::mkvs::{sync::HostReadSyncer, Root, Tree},
    types::HostStorageEndpoint,
    Protocol,
};

#[derive(Clone)]
pub struct ReadSyncer(Arc<Protocol>);

impl ReadSyncer {
    pub fn new(protocol: Arc<Protocol>) -> Self {
        Self(protocol)
    }
}

pub trait HostTree {
    fn tree(&self, root: Root) -> Tree;
}

impl HostTree for ReadSyncer {
    fn tree(&self, root: Root) -> Tree {
        let config = self.0.get_config();
        let read_syncer = HostReadSyncer::new(self.0.clone(), HostStorageEndpoint::Runtime);
        Tree::builder()
            .with_capacity(
                config.storage.cache_node_capacity,
                config.storage.cache_value_capacity,
            )
            .with_root(root)
            .build(Box::new(read_syncer))
    }
}

impl HostTree for &ReadSyncer {
    fn tree(&self, root: Root) -> Tree {
        let config = self.0.get_config();
        let read_syncer = HostReadSyncer::new(self.0.clone(), HostStorageEndpoint::Runtime);
        Tree::builder()
            .with_capacity(
                config.storage.cache_node_capacity,
                config.storage.cache_value_capacity,
            )
            .with_root(root)
            .build(Box::new(read_syncer))
    }
}
