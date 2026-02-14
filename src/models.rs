use chrono::{DateTime, Duration, Local};
use std::path::Path;
use uuid::Uuid;

#[derive(Debug)]
pub struct MemoId(Uuid);

#[derive(Debug)]
pub struct Memo {
    id: MemoId,
    content: String,
    created_date: DateTime<Local>,
    created_path: Path,
}

impl Memo {
    pub fn new(
        id: MemoId,
        content: String,
        created_date: DateTime<Local>,
        created_path: impl AsRef<Path>,
    ) -> Self {
        Self {
            id,
            content,
            created_date,
            created_path: *created_path.as_ref().iter().as_path(),
        }
    }
}
