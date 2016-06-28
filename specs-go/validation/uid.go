package validation

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

// Validate validates this UID
func (m *UID) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.Uint32.Validate(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
