interface Props {
  readonly label: string
  readonly inProgress: boolean
  readonly onCancel: () => void
  readonly onConfirm: () => void
}

export default function ConfirmDeleteFooter({ label, inProgress, onCancel, onConfirm }: Props) {
  return (
    <div className="pd-confirm-foot">
      <button
        className="pd-btn-ghost"
        onClick={onCancel}
        disabled={inProgress}
      >
        Cancel
      </button>
      <button
        className="pd-btn-confirm-delete"
        onClick={onConfirm}
        disabled={inProgress}
      >
        <svg
          width="12"
          height="12"
          viewBox="0 0 12 12"
          fill="none"
          stroke="currentColor"
          strokeWidth="1.8"
        >
          <path d="M1.5 3.5h9M4 3.5V2a.5.5 0 01.5-.5h3a.5.5 0 01.5.5v1.5M5 5.5v3M7 5.5v3M2.5 3.5l.5 7a.5.5 0 00.5.5h6a.5.5 0 00.5-.5l.5-7" />
        </svg>
        {inProgress ? 'Deleting…' : label}
      </button>
    </div>
  )
}
