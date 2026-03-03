// Migration: Xóa duplicate pc_pos_orders theo (orderId, ownerOrganizationId)
// Chạy TRƯỚC KHI restart server (vì unique index idx_pos_order_unique sẽ fail nếu còn duplicate)
//
// Cách chạy: mongosh <connection_uri> <db_name> --file scripts/migration_deduplicate_pos_orders.js
// Hoặc: node scripts/run_migration.js migration_deduplicate_pos_orders

const coll = db.getCollection("pc_pos_orders");

// Aggregate để tìm các nhóm (orderId, ownerOrganizationId) có > 1 document
const duplicates = coll.aggregate([
  { $match: { orderId: { $exists: true }, ownerOrganizationId: { $exists: true } } },
  { $group: { _id: { orderId: "$orderId", ownerOrganizationId: "$ownerOrganizationId" }, count: { $sum: 1 }, ids: { $push: "$_id" } } },
  { $match: { count: { $gt: 1 } } }
]).toArray();

if (duplicates.length === 0) {
  print("Không có duplicate. Không cần thực hiện migration.");
  quit(0);
}

print("Tìm thấy " + duplicates.length + " nhóm duplicate:");
let totalDeleted = 0;
duplicates.forEach(g => {
  print("  orderId=" + g._id.orderId + ", ownerOrgId=" + g._id.ownerOrganizationId + ": " + g.count + " bản ghi");
  // Giữ bản ghi có updatedAt lớn nhất (mới nhất), xóa các bản còn lại
  const ids = g.ids;
  const docs = coll.find({ _id: { $in: ids } }).sort({ updatedAt: -1 }).toArray();
  const keepId = docs[0]._id;
  const toDelete = docs.slice(1);
  toDelete.forEach(d => {
    coll.deleteOne({ _id: d._id });
    totalDeleted++;
    print("    Đã xóa _id: " + d._id);
  });
});

print("Tổng cộng đã xóa " + totalDeleted + " bản ghi duplicate.");
print("Có thể restart server để tạo unique index idx_pos_order_unique.");
