// Migration Script: Cleanup DraftApproval collection (đã bỏ bước approval riêng)
// Chạy script này trong MongoDB shell hoặc MongoDB Compass
// Usage: mongo <database_name> migration_cleanup_draft_approvals.js
//
// LƯU Ý:
// - Script này XÓA collection content_draft_approvals (đã không dùng nữa)
// - Backup data trước khi chạy migration
// - Chỉ chạy sau khi đã migrate approval status sang draft nodes

print("🚀 Bắt đầu cleanup: DraftApproval collection");
print("==========================================");

const collectionName = "content_draft_approvals";

try {
    const collection = db.getCollection(collectionName);
    
    // Kiểm tra collection có tồn tại không
    if (!collection) {
        print(`⚠️  Collection ${collectionName} không tồn tại, không cần cleanup`);
        print("✅ Cleanup hoàn tất!");
        quit(0);
    }
    
    // Đếm số documents
    const count = collection.countDocuments({});
    print(`📊 Collection: ${collectionName}`);
    print(`   - Tổng số documents: ${count}`);
    
    if (count === 0) {
        print(`   ✅ Collection đã rỗng, không cần cleanup`);
        print("✅ Cleanup hoàn tất!");
        quit(0);
    }
    
    // Hỏi xác nhận (trong MongoDB shell, có thể bỏ qua nếu dùng script tự động)
    print(`\n⚠️  CẢNH BÁO: Script này sẽ XÓA ${count} documents trong collection ${collectionName}`);
    print("   Nếu muốn backup trước, dừng script này và export data trước.");
    print("   Để tiếp tục, uncomment dòng drop() bên dưới và chạy lại.");
    
    // UNCOMMENT DÒNG NÀY ĐỂ THỰC SỰ XÓA COLLECTION:
    // collection.drop();
    
    // HOẶC XÓA TỪNG DOCUMENT (an toàn hơn):
    // const result = collection.deleteMany({});
    // print(`   ✅ Đã xóa: ${result.deletedCount} documents`);
    
    print("\n==========================================");
    print("📝 HƯỚNG DẪN:");
    print("   1. Backup data: mongoexport --db=<db> --collection=content_draft_approvals --out=backup.json");
    print("   2. Uncomment dòng drop() hoặc deleteMany() ở trên");
    print("   3. Chạy lại script này");
    print("==========================================");
    
} catch (error) {
    print(`❌ Lỗi: ${error.message}`);
    quit(1);
}

print("✅ Cleanup script hoàn tất (chưa thực thi xóa, cần uncomment để chạy)");
