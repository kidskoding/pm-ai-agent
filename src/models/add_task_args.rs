use serde::{Deserialize, Serialize};

use crate::models::priority::Priority;

#[derive(Serialize, Deserialize)]
pub struct AddTaskArgs {
    pub title: String,
    pub user_story: String,
    pub priority: Priority
}