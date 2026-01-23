use serde::{Deserialize, Serialize};

#[derive(Serialize, Deserialize)]
pub enum Priority {
    LOW,
    MEDIUM,
    HIGH
}