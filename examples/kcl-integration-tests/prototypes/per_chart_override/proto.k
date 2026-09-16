# Default configuration of the per_chart_override prototype. Applications instantiate this
# schema instead of repeating its values; the directory name is a KCL identifier so that
# `prototypes.per_chart_override` is importable.
import myks as m

schema PerChartOverride(m.App):
    proto: str = "per_chart_override"
    helm?: {str:any} = {
        charts = [
            {name = "render-test-1", releaseName = "overridden-release-name-1"}
            {name = "render-test-2", releaseName = "overridden-release-name-2"}
        ]
    }
