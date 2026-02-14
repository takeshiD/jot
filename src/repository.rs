use crate::models::MemoId;

pub trait MemoRepository {
    fn insert();
    fn remove();
    fn find_by_id(id: MemoId) -> bool;
}
