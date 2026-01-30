// Migration Script: Đổi type "layer" → "pillar" trong content nodes và draft nodes
// Chạy script này trong MongoDB shell hoặc MongoDB Compass
// Usage: mongo <database_name> migration_layer_to_pillar.js
//
// LƯU Ý:
// - Script này đổi type value từ "layer" → "pillar" trong DB
// - Backup data trước khi chạy migration
// - Kiểm tra kết quả sau khi chạy

print("🚀 Bắt đầu migration: layer → pillar");
print("==========================================");

// Danh sách collections cần migrate
const collections = [
    "content_nodes",           // Production content nodes
    "content_draft_nodes"      // Draft content nodes
];

let totalUpdated = 0;
let totalErrors = 0;

collections.forEach(collectionName => {
    try {
        const collection = db.getCollection(collectionName);
        
        // Kiểm tra collection có tồn tại không
        if (!collection) {
            print(`⚠️  Collection ${collectionName} không tồn tại, bỏ qua`);
            return;
        }
        
        // Đếm số documents có type = "layer"
        const countBefore = collection.countDocuments({ type: "layer" });
        print(`\n📊 Collection: ${collectionName}`);
        print(`   - Số documents có type = "layer": ${countBefore}`);
        
        if (countBefore === 0) {
            print(`   ✅ Không có document nào cần migrate`);
            return;
        }
        
        // Update: đổi type từ "layer" → "pillar"
        const result = collection.updateMany(
            { type: "layer" },
            { $set: { type: "pillar" } }
        );
        
        print(`   ✅ Đã update: ${result.modifiedCount} documents`);
        totalUpdated += result.modifiedCount;
        
        // Verify: kiểm tra lại
        const countAfter = collection.countDocuments({ type: "layer" });
        const countPillar = collection.countDocuments({ type: "pillar" });
        
        if (countAfter > 0) {
            print(`   ⚠️  Cảnh báo: Vẫn còn ${countAfter} documents có type = "layer"`);
            totalErrors += countAfter;
        } else {
            print(`   ✅ Verify: Không còn document nào có type = "layer"`);
            print(`   ✅ Verify: Có ${countPillar} documents có type = "pillar"`);
        }
        
    } catch (error) {
        print(`   ❌ Lỗi khi migrate collection ${collectionName}: ${error.message}`);
        totalErrors++;
    }
});

print("\n==========================================");
print("📊 TỔNG KẾT:");
print(`   ✅ Tổng số documents đã update: ${totalUpdated}`);
if (totalErrors > 0) {
    print(`   ⚠️  Tổng số lỗi/cảnh báo: ${totalErrors}`);
} else {
    print(`   ✅ Không có lỗi`);
}
print("==========================================");
print("✅ Migration hoàn tất!");
