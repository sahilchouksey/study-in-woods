# PYQ Crawler Integration - Testing Guide

**Status**: ✅ Implementation Complete - Ready for Testing  
**Date**: December 14, 2024  
**Services**: Backend (http://localhost:8080) & Frontend (http://localhost:3000) - Both Running

---

## Pre-Testing Checklist

- [x] TypeScript compilation passes (0 errors)
- [x] Backend API is running on port 8080
- [x] Frontend dev server is running on port 3000
- [ ] User is logged in to the application
- [ ] At least one subject exists in the system
- [ ] Subject has a valid subject code (e.g., MCA-301)

---

## Test Scenarios

### 1. UI Integration Test

**Objective**: Verify new section appears in PYQ tab

**Steps**:
1. Open browser to http://localhost:3000
2. Log in with your credentials
3. Navigate to: **Courses** → Select a course → Select a subject
4. Click on the subject card to open the dialog
5. Click on the **"PYQs"** tab

**Expected Result**:
```
✅ Should see existing PYQ papers list at top (if any)
✅ Should see a visual divider with "Search External Papers" label
✅ Should see "Available Papers from RGPV Online" section
✅ Should see "Search Available Papers" button (not yet clicked)
✅ Should NOT see any results yet (search not triggered)
```

**Screenshot Location**: Compare with mockup in PYQ_CRAWLER_INTEGRATION.md

---

### 2. Search Functionality Test (Without Filters)

**Objective**: Test basic search functionality

**Steps**:
1. In the PYQs tab, locate the "Available Papers from RGPV Online" section
2. Click the **"Search Available Papers"** button (blue button)
3. Wait for results to load

**Expected Result**:
```
✅ Button should disable and show "Searching..." with spinner
✅ Should see skeleton loading cards (3 shimmer cards)
✅ After 2-5 seconds, should see results appear
✅ Each result card should show:
   - Paper title (e.g., "MCA-301-DATA-MINING-DEC-2024")
   - Year badge (e.g., "2024")
   - Month badge (e.g., "December")
   - Exam type badge (e.g., "End Sem")
   - Source badge (e.g., "RGPV Online")
   - Either "Add" button (blue) OR "Added" button (gray, disabled)
✅ Should see statistics: "Found X available (Y already added)"
```

**If No Results**:
```
⚠️ Should see empty state message:
   "No papers found. Try adjusting your filters or check back later."
```

**Common Issues**:
- Network error → Check backend logs
- CORS error → Check browser console (should be allowed)
- 500 error → Check backend logs for crawler errors

---

### 3. Search with Filters Test

**Objective**: Test filter functionality

**Steps**:
1. Click the **"Filters ▼"** button
2. Dropdown should expand showing filter inputs
3. Set filters:
   - **Course**: MCA (default)
   - **Year**: 2024
   - **Month**: December
4. Click **"Search Available Papers"** again
5. Wait for results

**Expected Result**:
```
✅ Filter dropdown expands/collapses smoothly
✅ Search uses filter parameters
✅ Results should be filtered by year and month
✅ Only papers matching filters should appear
```

**Try Different Combinations**:
- Year: 2023, Month: June
- Year: 2024, Month: (empty - all months)
- Course: (different course if available)

---

### 4. Ingestion Workflow Test

**Objective**: Test adding a new PYQ paper

**Steps**:
1. Search for available papers (as in Test 2)
2. Find a paper with **"Add"** button (blue, not disabled)
3. Click the **"Add"** button
4. Wait for ingestion to complete

**Expected Result**:
```
✅ Button should change to "Adding..." with spinner
✅ Should see success toast: "PYQ paper ingested successfully!"
✅ Button should change to "Added" (gray, disabled with checkmark)
✅ Statistics should update: "Y already added" count increases by 1
✅ Paper should appear in the main PYQ list at top (after extraction completes)
```

**Timeline**:
- Ingestion API call: ~2-5 seconds
- Backend processing (download PDF, upload to Spaces): ~10-30 seconds
- Extraction (OCR + AI): ~1-3 minutes (happens in background)

**Verification**:
- Refresh the page after 2-3 minutes
- Navigate back to PYQs tab
- Paper should now appear in main list with status "Ready" or "Processing"

---

### 5. Deduplication Test

**Objective**: Verify already-ingested papers show "Added" state

**Steps**:
1. After successfully ingesting a paper (Test 4)
2. Perform search again with same filters
3. Locate the paper you just ingested

**Expected Result**:
```
✅ Previously ingested paper should show "Added" button (disabled, gray)
✅ Button should have checkmark icon
✅ Hovering should NOT change cursor to pointer
✅ Clicking should do nothing (disabled state)
```

**Deduplication Logic**:
- Papers are matched by `year-month` combination
- Example: "2024-December" → If already in main list, shows "Added"

---

### 6. Error Handling Tests

**Objective**: Test error states

#### 6.1 Network Error Test
**Steps**:
1. Stop the backend API (`Ctrl+C` in backend terminal)
2. Try searching for papers

**Expected Result**:
```
✅ Should see error message: "Failed to search available PYQs"
✅ Should see retry option or clear error state
```

#### 6.2 Empty Results Test
**Steps**:
1. Restart backend
2. Set filters to unlikely combination:
   - Year: 1990
   - Month: January
3. Search

**Expected Result**:
```
✅ Should see empty state message (not an error)
✅ Should see: "No papers found. Try adjusting your filters..."
```

#### 6.3 Ingestion Failure Test
**Steps**:
1. Open browser DevTools → Network tab
2. Add a paper
3. While ingesting, stop backend API

**Expected Result**:
```
✅ Should see error toast: "Failed to ingest PYQ paper"
✅ Button should revert to "Add" state (not stuck in "Adding...")
✅ Can retry after restarting backend
```

---

### 7. Edge Cases

#### 7.1 Rapid Click Test
**Steps**:
1. Search for papers
2. Quickly click "Add" button multiple times (double/triple click)

**Expected Result**:
```
✅ Button should disable immediately after first click
✅ Should only send ONE ingestion request (not multiple)
✅ Should not cause UI glitches or duplicate papers
```

#### 7.2 Page Refresh During Ingestion
**Steps**:
1. Start ingesting a paper
2. While button shows "Adding...", refresh the page (F5)
3. Navigate back to PYQs tab

**Expected Result**:
```
✅ UI should recover gracefully
✅ Re-searching should show correct state (Added or Add)
✅ Backend should complete ingestion regardless
```

#### 7.3 Concurrent Ingestion
**Steps**:
1. Search for papers
2. Click "Add" on first paper
3. Immediately click "Add" on second paper (don't wait)

**Expected Result**:
```
✅ Both buttons should disable independently
✅ Both papers should ingest successfully
✅ Both should show "Added" state after completion
```

---

## API Testing (Optional - For Debugging)

### Test Search Endpoint

```bash
# Replace {subject_id} with actual ID
curl -X GET "http://localhost:8080/api/v1/subjects/{subject_id}/pyqs/search-available?course=MCA&year=2024&month=December" \
  -H "Content-Type: application/json"
```

**Expected Response**:
```json
{
  "success": true,
  "data": {
    "available_papers": [
      {
        "pdf_url": "https://...",
        "title": "MCA-301-DATA-MINING-DEC-2024",
        "year": 2024,
        "month": "December",
        "exam_type": "End Sem",
        "source": "rgpv_online"
      }
    ],
    "available_count": 5,
    "ingested_count": 2
  }
}
```

### Test Ingestion Endpoint

```bash
# Replace {subject_id} with actual ID
curl -X POST "http://localhost:8080/api/v1/subjects/{subject_id}/pyqs/ingest" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "pdf_url": "https://example.com/paper.pdf",
    "title": "MCA-301-DATA-MINING-DEC-2024",
    "year": 2024,
    "month": "December",
    "exam_type": "End Sem",
    "source": "rgpv_online"
  }'
```

**Expected Response**:
```json
{
  "success": true,
  "data": {
    "id": 123,
    "title": "MCA-301-DATA-MINING-DEC-2024",
    "status": "pending_extraction",
    "created_at": "2024-12-14T10:30:00Z"
  }
}
```

### Test Crawler Sources Endpoint

```bash
curl -X GET "http://localhost:8080/api/v1/pyqs/crawler-sources" \
  -H "Content-Type: application/json"
```

**Expected Response**:
```json
{
  "success": true,
  "data": {
    "sources": [
      {
        "id": "rgpv_online",
        "name": "RGPV Online",
        "description": "RGPV University Online Exam Portal",
        "supported_courses": ["MCA", "MTech"],
        "available": true
      }
    ]
  }
}
```

---

## Known Limitations

### Current Implementation
1. **Deduplication**: Only matches by `year-month`, doesn't distinguish exam types
2. **Single Source**: Only RGPV Online crawler implemented
3. **No Pagination**: Results limited to backend's default (typically 50-100)
4. **No Bulk Actions**: Must ingest papers one at a time
5. **No Preview**: Can't preview PDF before ingesting

### Future Enhancements (Phase 2)
- [ ] Add exam type to deduplication logic
- [ ] Implement AKTU, VTU crawlers (factory pattern ready)
- [ ] Add pagination for >20 results
- [ ] Add "Ingest All" button
- [ ] Add PDF preview modal
- [ ] Add progress bar during ingestion
- [ ] Persist filter preferences in localStorage

---

## Troubleshooting

### Issue: "No papers found" on first search
**Cause**: RGPV website might be down or crawler needs update  
**Solution**: 
1. Check backend logs for crawler errors
2. Try different year/month combinations
3. Verify RGPV Online website is accessible

### Issue: Search never completes (infinite loading)
**Cause**: Backend timeout or network issue  
**Solution**:
1. Check browser Network tab for failed requests
2. Check backend logs for errors
3. Increase request timeout if needed

### Issue: "Added" button shows but paper not in main list
**Cause**: Extraction still in progress or failed  
**Solution**:
1. Wait 2-3 minutes for extraction to complete
2. Refresh the page
3. Check backend logs for extraction errors
4. Verify OCR service is running

### Issue: CORS errors in browser console
**Cause**: Frontend not in allowed origins  
**Solution**:
1. Check backend `ALLOWED_ORIGINS` env variable
2. Should include `http://localhost:3000`
3. Restart backend after changing

### Issue: TypeScript errors in IDE but code runs fine
**Cause**: Language server cache not refreshed  
**Solution**:
1. Run `npx tsc --noEmit` to verify (should show 0 errors)
2. Restart VS Code or reload window
3. Safe to ignore if compilation succeeds

---

## Success Criteria

**All Tests Pass** ✅ When:
- [ ] New section appears in PYQs tab
- [ ] Search functionality works without filters
- [ ] Search functionality works with filters
- [ ] Can successfully ingest a new paper
- [ ] Ingested papers show "Added" state on re-search
- [ ] Error states display properly
- [ ] No console errors or warnings
- [ ] UI is responsive and smooth

**Ready for Production** 🚀 When:
- [ ] All success criteria met
- [ ] Edge cases handled gracefully
- [ ] Performance is acceptable (<5s search time)
- [ ] Backend logs show no errors
- [ ] Code is documented and clean

---

## Next Steps After Testing

### If All Tests Pass:
1. ✅ Mark implementation as complete
2. 📄 Update main documentation
3. 🎉 Deploy to staging environment
4. 🔍 User acceptance testing

### If Issues Found:
1. 🐛 Document bugs in new file: `BUGS_PYQ_CRAWLER.md`
2. 🔧 Create fixes for each bug
3. ✅ Re-test after fixes
4. 📝 Update this guide with lessons learned

---

## Contact & Support

**Implementation By**: AI Assistant  
**Documentation**: `/PYQ_CRAWLER_INTEGRATION.md`  
**Backend Docs**: `/apps/api/services/pyq_crawler/README.md`  
**Issues**: Document in project issue tracker

---

**Happy Testing!** 🧪
