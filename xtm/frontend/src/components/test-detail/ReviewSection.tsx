import { REVIEW_ENABLED } from "../../features";
import { verdictLabel, DirtyDot } from "./rows";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

// ReviewSection records a review verdict, shown only while REVIEW_ENABLED.
export function ReviewSection({ s, a, p }: { s: TestDetailState; a: TestDetailActions; p: TestDetailProps }) {
  const { reviewer, setReviewer, reviewNote, setReviewNote, review, isDirty } = s;
  const { setVerdict } = a;
  const { readOnly } = p;
  return (
    <>
      {REVIEW_ENABLED && (
        <>
      <h4>
        Review {isDirty("review") && <DirtyDot />}
      </h4>
      <div className="review-box">
        <div className="review-state">
          <span
            className={`review-verdict review-${review?.verdict || "none"}`}
          >
            {verdictLabel(review?.verdict)}
          </span>
          {review?.verdict && (review.reviewer || review.reviewedAt) && (
            <span className="muted review-meta">
              {review.reviewer ? `by ${review.reviewer}` : ""}
              {review.reviewedAt
                ? ` · ${new Date(review.reviewedAt).toLocaleDateString()}`
                : ""}
            </span>
          )}
        </div>
        {!readOnly && (
          <>
            <input
              className="detail-input detail-input-inline review-reviewer"
              value={reviewer}
              onChange={(e) => setReviewer(e.target.value)}
              placeholder="Reviewer name"
            />
            <input
              className="detail-input detail-input-inline review-note"
              value={reviewNote}
              onChange={(e) => setReviewNote(e.target.value)}
              placeholder="Review note (optional)"
            />
            <div className="review-actions">
              <button
                className="btn review-approve"
                onClick={() => setVerdict("approved")}
              >
                Approve
              </button>
              <button
                className="btn review-reject"
                onClick={() => setVerdict("rejected")}
              >
                Reject
              </button>
              <button className="btn" onClick={() => setVerdict("pending")}>
                Pending
              </button>
              {review?.verdict && (
                <button
                  className="btn btn-ghost review-clear"
                  onClick={() => setVerdict("")}
                  title="Clear the review"
                >
                  Clear
                </button>
              )}
            </div>
          </>
        )}
      </div>
        </>
      )}
    </>
  );
}
