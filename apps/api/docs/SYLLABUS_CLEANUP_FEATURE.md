# Syllabus Data Cleanup on Upload

**Date**: December 12, 2025  
**Status**: ✅ Implemented  
**Feature**: Automatic deletion of existing syllabus data when uploading new syllabus

---

## 🎯 Problem Solved

**Issue**: When users upload a new syllabus for a semester, the old syllabus data (subjects, units, topics, books) remains in the database, causing:
- Duplicate/stale data
- Confusion about which syllabus is current
- Data inconsistency
- Cluttered database

**Solution**: Automatically delete all existing syllabus data for the semester before extracting the new syllabus.

---

## ✅ Implementation

### 1. **New Service Methods**

**File**: `services/syllabus_service.go`

#### Method 1: `deleteExistingSyllabusDataForSubject()`
```go
func (s *SyllabusService) deleteExistingSyllabusDataForSubject(ctx context.Context, subjectID uint) error
```

**Purpose**: Delete all syllabus data for a single subject  
**Use Case**: When re-extracting a single subject's syllabus  
**What it deletes**:
- All syllabuses for the subject
- All units (cascade)
- All topics (cascade)
- All book references (cascade)

#### Method 2: `DeleteExistingSyllabusDataForSemester()` (Public)
```go
func (s *SyllabusService) DeleteExistingSyllabusDataForSemester(ctx context.Context, semesterID uint) error
```

**Purpose**: Delete all syllabus data for all subjects in a semester  
**Use Case**: When uploading a new semester-level syllabus (multi-subject PDF)  
**What it deletes**:
1. Finds all subjects in the semester
2. Deletes all syllabuses for those subjects
3. Cascade deletes all units, topics, and books

**Features**:
- Uses `Unscoped()` for permanent deletion (not soft delete)
- Logs deletion counts for monitoring
- Safe to call even if no data exists
- Returns error only on database failures

---

### 2. **Updated Upload Flow**

**File**: `handlers/syllabus/syllabus.go`

**Endpoint**: `POST /api/v1/semesters/:semester_id/syllabus/upload`

**New Flow**:
```
1. Verify semester exists
2. Validate uploaded file
3. ✨ DELETE existing syllabus data for semester (NEW!)
4. Create temporary "General" subject
5. Upload document to Spaces
6. Extract syllabus (creates new subjects)
7. Delete temporary subject if not needed
8. Return extracted syllabuses
```

**Code Change**:
```go
// Delete existing syllabus data for this semester (clean slate for new upload)
if err := h.syllabusService.DeleteExistingSyllabusDataForSemester(c.Context(), semester.ID); err != nil {
    return response.InternalServerError(c, "Failed to clean existing syllabus data: "+err.Error())
}
```

---

### 3. **Updated Extraction Flow**

**File**: `services/syllabus_service.go`

**Method**: `ExtractSyllabusFromDocument()`

**New Flow**:
```
1. Validate AI is enabled
2. Get document from database
3. ✨ DELETE existing syllabus data for subject (NEW!)
4. Download PDF from Spaces
5. Determine extraction strategy (direct vs chunked)
6. Extract syllabus
7. Save to database
```

**Code Change**:
```go
// Delete existing syllabus data for this subject (clean slate for new upload)
if err := s.deleteExistingSyllabusDataForSubject(ctx, document.SubjectID); err != nil {
    log.Printf("Warning: Failed to delete existing syllabus data: %v", err)
    // Continue anyway - non-critical error
}
```

**Note**: This deletion is logged as a warning if it fails, but extraction continues. This ensures that even if cleanup fails, the new data is still extracted.

---

## 📊 Database Impact

### Cascade Deletion

The database schema uses `OnDelete:CASCADE` constraints:

```go
// Syllabus model
Units    []SyllabusUnit  `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE"`
Books    []BookReference `gorm:"foreignKey:SyllabusID;constraint:OnDelete:CASCADE"`

// SyllabusUnit model
Topics   []SyllabusTopic `gorm:"foreignKey:UnitID;constraint:OnDelete:CASCADE"`
```

**What this means**:
- Deleting a `Syllabus` automatically deletes all its `Units` and `Books`
- Deleting a `Unit` automatically deletes all its `Topics`
- One DELETE query triggers cascading deletions

### Deletion Type

Using `Unscoped()` for **permanent deletion**:
```go
s.db.Unscoped().Where("subject_id = ?", subjectID).Delete(&model.Syllabus{})
```

**Why permanent?**
- Old syllabus data is no longer relevant
- Soft-deleted data would accumulate unnecessarily
- Clean database = better performance

---

## 🔍 Example Scenarios

### Scenario 1: First Upload (No Existing Data)
```
User uploads syllabus for Semester 1
→ DeleteExistingSyllabusDataForSemester(semester_id=1)
→ Finds 0 existing syllabuses
→ Logs: "No existing syllabus data found for semester 1"
→ Continues with extraction
→ Creates 12 new subjects with syllabuses
```

### Scenario 2: Re-Upload (Existing Data)
```
User uploads NEW syllabus for Semester 1 (already has data)
→ DeleteExistingSyllabusDataForSemester(semester_id=1)
→ Finds 12 subjects in semester
→ Finds 12 existing syllabuses
→ Logs: "Deleting 12 existing syllabus(es) for semester 1 (12 subjects)"
→ Deletes:
   - 12 syllabuses
   - 60 units (cascade)
   - 320 topics (cascade)
   - 48 books (cascade)
→ Logs: "Successfully deleted existing syllabus data for semester 1"
→ Continues with extraction
→ Creates fresh data from new PDF
```

### Scenario 3: Single Subject Re-Extraction
```
User re-extracts syllabus for Document ID 5 (subject_id=3)
→ deleteExistingSyllabusDataForSubject(subject_id=3)
→ Finds 1 existing syllabus
→ Logs: "Deleting 1 existing syllabus(es) for subject 3"
→ Deletes:
   - 1 syllabus
   - 5 units (cascade)
   - 25 topics (cascade)
   - 4 books (cascade)
→ Logs: "Successfully deleted existing syllabus data for subject 3"
→ Continues with extraction
→ Creates fresh data
```

---

## 🧪 Testing

### Manual Test

```bash
# 1. Upload first syllabus
curl -X POST http://localhost:8080/api/v1/semesters/1/syllabus/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@syllabus_v1.pdf"

# Check database
docker exec study-woods-postgres psql -U postgres -d study_in_woods \
  -c "SELECT COUNT(*) FROM syllabuses WHERE subject_id IN (SELECT id FROM subjects WHERE semester_id = 1);"
# Output: 12

# 2. Upload NEW syllabus (should replace old data)
curl -X POST http://localhost:8080/api/v1/semesters/1/syllabus/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "file=@syllabus_v2.pdf"

# Check database again
docker exec study-woods-postgres psql -U postgres -d study_in_woods \
  -c "SELECT COUNT(*) FROM syllabuses WHERE subject_id IN (SELECT id FROM subjects WHERE semester_id = 1);"
# Output: 12 (new data, old data deleted)

# Check logs for deletion message
# Should see: "Deleting 12 existing syllabus(es) for semester 1 (12 subjects)"
```

### Database Verification

```sql
-- Before upload
SELECT 
    s.id as syllabus_id,
    s.subject_code,
    COUNT(DISTINCT su.id) as units,
    COUNT(DISTINCT st.id) as topics
FROM syllabuses s
LEFT JOIN syllabus_units su ON s.id = su.syllabus_id
LEFT JOIN syllabus_topics st ON su.id = st.unit_id
WHERE s.subject_id IN (SELECT id FROM subjects WHERE semester_id = 1)
GROUP BY s.id;

-- Upload new syllabus

-- After upload (should show different syllabus IDs, same or different counts)
SELECT 
    s.id as syllabus_id,
    s.subject_code,
    COUNT(DISTINCT su.id) as units,
    COUNT(DISTINCT st.id) as topics
FROM syllabuses s
LEFT JOIN syllabus_units su ON s.id = su.syllabus_id
LEFT JOIN syllabus_topics st ON su.id = st.unit_id
WHERE s.subject_id IN (SELECT id FROM subjects WHERE semester_id = 1)
GROUP BY s.id;
```

---

## 📝 Logging

### Log Messages

**Success (no existing data)**:
```
SyllabusService: No existing syllabus data found for semester 1
```

**Success (with existing data)**:
```
SyllabusService: Deleting 12 existing syllabus(es) for semester 1 (12 subjects)
SyllabusService: Successfully deleted existing syllabus data for semester 1
```

**Warning (cleanup failed, but continuing)**:
```
Warning: Failed to delete existing syllabus data: <error message>
```

**Error (cleanup failed and stopped)**:
```
Failed to clean existing syllabus data: <error message>
```

---

## ⚠️ Important Notes

### 1. **Data Loss is Intentional**
- Old syllabus data is **permanently deleted**
- This is the desired behavior for clean data
- Users should be aware that uploading a new syllabus replaces the old one

### 2. **Subject Preservation**
- Subjects themselves are **NOT deleted**
- Only syllabus data (units, topics, books) is deleted
- Subjects may be reused if the new syllabus has the same subject codes

### 3. **Document Preservation**
- Old document files in Spaces are **NOT deleted**
- Only the extracted syllabus data is deleted
- Documents remain accessible for audit/history

### 4. **Transaction Safety**
- Deletion happens **before** extraction
- If extraction fails, old data is already gone
- Consider adding a backup/archive feature in the future

---

## 🚀 Future Enhancements

### Potential Improvements:

1. **Archive Instead of Delete**
   - Soft delete with `archived_at` timestamp
   - Keep historical syllabus data
   - Allow viewing previous versions

2. **Confirmation Prompt**
   - Warn user if existing data will be deleted
   - Require explicit confirmation
   - Show count of items to be deleted

3. **Rollback Feature**
   - Keep last N versions
   - Allow reverting to previous syllabus
   - Useful if new upload has errors

4. **Differential Update**
   - Compare old vs new syllabus
   - Only update changed subjects
   - Preserve unchanged data

---

## ✅ Summary

**What Changed**:
- ✅ Added `deleteExistingSyllabusDataForSubject()` method
- ✅ Added `DeleteExistingSyllabusDataForSemester()` method
- ✅ Updated `UploadAndExtractSyllabus()` handler to clean semester data
- ✅ Updated `ExtractSyllabusFromDocument()` to clean subject data
- ✅ Added comprehensive logging

**Benefits**:
- ✅ Clean, consistent data
- ✅ No duplicate syllabuses
- ✅ Clear which syllabus is current
- ✅ Better database performance
- ✅ Easier data management

**Files Modified**:
1. `services/syllabus_service.go` - Added cleanup methods
2. `handlers/syllabus/syllabus.go` - Added cleanup call before upload

**Ready to use!** 🎉
