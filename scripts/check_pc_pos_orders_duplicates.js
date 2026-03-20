// Script kiểm tra duplicate trong pc_pos_orders
// Chạy: mongosh <connection_uri> <db_name> --file scripts/check_pc_pos_orders_duplicates.js
//
// Phân tích:
// - Duplicate theo (orderId, ownerOrganizationId): vi phạm business key
// - Duplicate theo orderId only: có thể do ownerOrganizationId khác nhau hoặc null

const coll = db.getCollection("pc_pos_orders");

print("=== 1. Tổng quan ===\n");
const total = coll.countDocuments({});
print("Tổng số document: " + total);

print("\n=== 2. Duplicate theo (orderId, ownerOrganizationId) ===\n");
const dupByKey = coll.aggregate([
  { $match: { orderId: { $exists: true } } },
  { $group: { _id: { orderId: "$orderId", ownerOrganizationId: "$ownerOrganizationId" }, count: { $sum: 1 }, ids: { $push: "$_id" } } },
  { $match: { count: { $gt: 1 } } },
  { $sort: { count: -1 } }
]).toArray();

if (dupByKey.length === 0) {
  print("Không có duplicate theo (orderId, ownerOrganizationId).");
} else {
  print("PHÁT HIỆN " + dupByKey.length + " nhóm duplicate:");
  dupByKey.forEach((g, i) => {
    if (i < 10) {
      print("  orderId=" + g._id.orderId + ", ownerOrgId=" + g._id.ownerOrganizationId + ": " + g.count + " bản ghi");
    }
  });
  if (dupByKey.length > 10) {
    print("  ... và " + (dupByKey.length - 10) + " nhóm khác");
  }
  const totalDupDocs = dupByKey.reduce((s, g) => s + g.count, 0);
  print("Tổng document duplicate: " + totalDupDocs);
}

print("\n=== 3. Duplicate theo orderId only (bỏ qua ownerOrg) ===\n");
const dupByOrderId = coll.aggregate([
  { $match: { orderId: { $exists: true, $ne: null } } },
  { $group: { _id: "$orderId", count: { $sum: 1 }, ownerOrgs: { $addToSet: "$ownerOrganizationId" } } },
  { $match: { count: { $gt: 1 } } },
  { $sort: { count: -1 } },
  { $limit: 20 }
]).toArray();

if (dupByOrderId.length === 0) {
  print("Không có orderId nào xuất hiện > 1 lần.");
} else {
  print("Các orderId xuất hiện nhiều lần (top 20):");
  dupByOrderId.forEach(g => {
    const orgs = g.ownerOrgs.filter(x => x != null).length;
    const nulls = g.ownerOrgs.filter(x => x == null).length;
    print("  orderId=" + g._id + ": " + g.count + " docs (ownerOrg: " + orgs + " có giá trị, " + nulls + " null)");
  });
}

print("\n=== 4. Document thiếu ownerOrganizationId ===\n");
const noOwnerOrg = coll.countDocuments({ orderId: { $exists: true }, $or: [{ ownerOrganizationId: { $exists: false } }, { ownerOrganizationId: null }] });
print("Số document có orderId nhưng thiếu/null ownerOrganizationId: " + noOwnerOrg);

print("\n=== 5. Index hiện tại ===\n");
const indexes = coll.getIndexes();
indexes.forEach(idx => {
  const unique = idx.unique ? " [UNIQUE]" : "";
  print("  " + idx.name + ": " + JSON.stringify(idx.key) + unique);
});

print("\n=== 6. Mẫu document duplicate (nếu có) ===\n");
if (dupByKey.length > 0) {
  const sample = dupByKey[0];
  const docs = coll.find({ _id: { $in: sample.ids } }).limit(3).toArray();
  docs.forEach((d, i) => {
    print("Doc " + (i + 1) + ": _id=" + d._id + ", orderId=" + d.orderId + ", ownerOrgId=" + d.ownerOrganizationId + ", posData.id=" + (d.posData && d.posData.id));
  });
}
