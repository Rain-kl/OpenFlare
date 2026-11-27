use crate::geo::ClientPrefixKey;
use hickory_proto::op::ResponseCode;
use hickory_proto::rr::{LowerName, Record, RecordType};
use moka::sync::Cache;
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

#[derive(Debug, Clone)]
struct Bucket {
    tokens: f64,
    last_refill: Instant,
}

/// Sharded token-bucket Response Rate Limiter per normalized client prefix.
pub struct PrefixRrl {
    qps: f64,
    burst: f64,
    shards: Vec<Mutex<Cache<ClientPrefixKey, Bucket>>>,
}

impl PrefixRrl {
    pub fn new(qps: u64, burst: u64) -> Self {
        let mut shards = Vec::with_capacity(64);
        for _ in 0..64 {
            shards.push(Mutex::new(
                Cache::builder()
                    .max_capacity(1024)
                    .time_to_idle(Duration::from_secs(60))
                    .build(),
            ));
        }
        Self {
            qps: qps as f64,
            burst: (burst.max(qps)) as f64,
            shards,
        }
    }

    pub fn allow(&self, key: ClientPrefixKey, now: Instant) -> bool {
        if self.qps <= 0.0 || self.burst <= 0.0 {
            return true;
        }
        let mut h = DefaultHasher::new();
        key.hash(&mut h);
        let idx = (h.finish() as usize) % self.shards.len();
        let Ok(guard) = self.shards[idx].lock() else {
            return true;
        };

        let mut bucket = guard.get(&key).unwrap_or(Bucket {
            tokens: self.burst,
            last_refill: now,
        });
        let elapsed = now
            .saturating_duration_since(bucket.last_refill)
            .as_secs_f64();
        if elapsed > 0.0 {
            bucket.tokens = (bucket.tokens + elapsed * self.qps).min(self.burst);
            bucket.last_refill = now;
        }
        let allowed = if bucket.tokens >= 1.0 {
            bucket.tokens -= 1.0;
            true
        } else {
            false
        };
        guard.insert(key, bucket);
        allowed
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct AnswerCacheKey {
    pub version: u64,
    pub qname: LowerName,
    pub qtype: RecordType,
    pub prefix: ClientPrefixKey,
    pub from_ecs: bool,
}

#[derive(Debug, Clone)]
pub struct CachedAnswer {
    pub rcode: ResponseCode,
    pub answers: Vec<Record>,
    pub soa: Vec<Record>,
    pub expires_at: Instant,
}

/// Bounded L1 answer cache storing pre-built DNS Records for sub-millisecond cache hits.
pub struct AnswerCache {
    inner: Cache<AnswerCacheKey, Arc<CachedAnswer>>,
}

impl AnswerCache {
    pub fn new(max_capacity: u64) -> Self {
        Self {
            inner: Cache::builder()
                .max_capacity(max_capacity)
                .time_to_live(Duration::from_secs(30))
                .build(),
        }
    }

    pub fn get_valid(&self, key: &AnswerCacheKey, now: Instant) -> Option<Arc<CachedAnswer>> {
        let entry = self.inner.get(key)?;
        if now < entry.expires_at {
            Some(entry)
        } else {
            None
        }
    }

    pub fn insert(&self, key: AnswerCacheKey, val: Arc<CachedAnswer>) {
        self.inner.insert(key, val);
    }
}
