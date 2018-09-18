module NoPipeline exposing (view, Msg)

import Html exposing (Html)
import Html.Attributes exposing (class, target, href)
import Html.Attributes.Aria exposing (ariaLabel)
import Concourse.Cli


type Msg
    = Noop


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
                , href "https://concourse-ci.org/setting-pipelines.html#fly-set-pipeline"
                ]
                [ Html.text "fly set-pipeline" ]
            , Html.text "` to set up your new pipeline"
            ]
        ]
