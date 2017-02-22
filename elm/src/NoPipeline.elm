module NoPipeline exposing (view, Msg, subscriptions)

import Html exposing (Html)
import Html.Attributes exposing (class, target, href)
import Html.Attributes.Aria exposing (ariaLabel)
import Time
import Concourse.Cli


type Msg
    = Tick Time.Time


view : Html Msg
view =
    Html.div [ class "display-in-middle" ]
        [ Html.div [ class "h1" ] [ Html.text "no pipelines configured" ]
        , Html.h3 [] [ Html.text "first, download the CLI tools:" ]
        , Html.ul [ class "cli-downloads" ]
            [ Html.li []
                [ Html.a
                    [ href (Concourse.Cli.downloadUrl "amd64" "darwin"), ariaLabel "Download OS X CLI" ]
                    [ Html.i [ class "fa fa-apple fa-5x" ] [] ]
                ]
            , Html.li []
                [ Html.a
                    [ href (Concourse.Cli.downloadUrl "amd64" "windows"), ariaLabel "Download Windows CLI" ]
                    [ Html.i [ class "fa fa-windows fa-5x" ] [] ]
                ]
            , Html.li []
                [ Html.a
                    [ href (Concourse.Cli.downloadUrl "amd64" "linux"), ariaLabel "Download Linux CLI" ]
                    [ Html.i [ class "fa fa-linux fa-5x" ] [] ]
                ]
            ]
        , Html.h3 []
            [ Html.text "then, use `"
            , Html.a
                [ class "ansi-blue-fg"
                , target "_blank"
                , href "https://concourse.ci/fly-set-pipeline.html"
                ]
                [ Html.text "fly set-pipeline" ]
            , Html.text "` to set up your new pipeline"
            ]
        ]


subscriptions : Sub Msg
subscriptions =
    Time.every (10 * Time.second) Tick
