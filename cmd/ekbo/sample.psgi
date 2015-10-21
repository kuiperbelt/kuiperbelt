use strict;
use warnings;
use utf8;
use Plack::Request;

my $app = sub {
    my $env = shift;
    my $req = Plack::Request->new($env);

    my $session = $req->parameters->{session};

    return ["200", ["X-Kuiperbelt-Session" => $session], [$session]];
};
