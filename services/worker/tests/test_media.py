import pytest

from audio_worker.media import calculate_peaks


def test_calculate_peaks_downsamples_pcm_to_signed_8_bit_pairs():
    peaks = calculate_peaks([-32768, -16384, 0, 16384, 32767], samples_per_point=2)

    assert peaks == [[-128, -64], [0, 64], [127, 127]]


def test_calculate_peaks_rejects_invalid_window():
    with pytest.raises(ValueError):
        calculate_peaks([0], samples_per_point=0)
