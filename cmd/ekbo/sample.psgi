use strict;
use warnings;
use utf8;
use Plack::Request;

my $app = sub {
    my $env = shift;
    my $req = Plack::Request->new($env);

    my $session = $req->header("X-Kuiperbelt-Session");

    return ["200", ["X-Kuiperbelt-Session" => $session], ["success connect"]];
};
